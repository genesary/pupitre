package handlers

import (
	"net"
	"sync"
	"testing"

	ber "github.com/go-asn1-ber/asn1-ber"
)

// LDAP protocol op application tags used by the mock.
const (
	appBindRequest        = 0
	appBindResponse       = 1
	appSearchRequest      = 3
	appSearchResultEntry  = 4
	appSearchResultDone   = 5
	ldapResultSuccess     = 0
	ldapResultInvalidCred = 49
)

// mockLDAP is a bare-bones LDAPv3 server: it answers simple binds and
// returns a single canned search entry. It speaks just enough of the
// protocol for the go-ldap client used by the handlers.
type mockLDAP struct {
	t        *testing.T
	ln       net.Listener
	userDN   string
	userPass string // the password the user bind must present to succeed
	bindDN   string // service-account DN (any password accepted)
	attrs    map[string][]string

	mu    sync.Mutex
	binds int // number of successful binds observed
}

// newMockLDAP starts the server on a random port and returns it. The caller
// closes it via t.Cleanup.
func newMockLDAP(t *testing.T) *mockLDAP {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	m := &mockLDAP{
		t: t, ln: ln,
		userDN:   "uid=jdoe,ou=people,dc=example,dc=com",
		userPass: "s3cret",
		bindDN:   "cn=svc,dc=example,dc=com",
		attrs: map[string][]string{
			"cn":          {"jdoe"},
			"mail":        {"jdoe@example.com"},
			"displayName": {"Jane Doe"},
		},
	}

	go m.serve()

	t.Cleanup(func() { _ = ln.Close() })

	return m
}

// url returns the ldap:// URL the client should dial.
func (m *mockLDAP) url() string {
	return "ldap://" + m.ln.Addr().String()
}

// serve accepts connections until the listener is closed.
func (m *mockLDAP) serve() {
	for {
		conn, err := m.ln.Accept()
		if err != nil {
			return
		}

		go m.handle(conn)
	}
}

// handle reads LDAP messages from one connection and answers them.
func (m *mockLDAP) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		packet, err := ber.ReadPacket(conn)
		if err != nil {
			return
		}

		if len(packet.Children) < 2 {
			return
		}

		msgID, ok := packet.Children[0].Value.(int64)
		if !ok {
			return
		}

		op := packet.Children[1]

		switch op.Tag { //nolint:exhaustive // only the three ops the client sends are handled
		case appBindRequest:
			m.replyBind(conn, msgID, op)
		case appSearchRequest:
			m.replySearch(conn, msgID)
		default: // appUnbindRequest and anything else: close
			return
		}
	}
}

// replyBind validates the credentials in a BindRequest and sends a
// BindResponse.
func (m *mockLDAP) replyBind(conn net.Conn, msgID int64, op *ber.Packet) {
	code := ldapResultInvalidCred

	if len(op.Children) >= 3 {
		dn, _ := op.Children[1].Value.(string)
		pw := op.Children[2].Data.String()

		if dn == m.bindDN || (dn == m.userDN && pw == m.userPass) {
			code = ldapResultSuccess

			m.mu.Lock()
			m.binds++
			m.mu.Unlock()
		}
	}

	resp := ber.Encode(ber.ClassApplication, ber.TypeConstructed, appBindResponse, nil, "Bind Response")
	resp.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, int64(code), "resultCode"))
	resp.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matchedDN"))
	resp.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnosticMessage"))

	m.send(conn, msgID, resp)
}

// replySearch sends one SearchResultEntry for the canned user followed by a
// successful SearchResultDone.
func (m *mockLDAP) replySearch(conn net.Conn, msgID int64) {
	entry := ber.Encode(ber.ClassApplication, ber.TypeConstructed, appSearchResultEntry, nil, "Search Result Entry")
	entry.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, m.userDN, "objectName"))

	attrList := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attributes")

	for name, values := range m.attrs {
		attr := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attribute")
		attr.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, name, "type"))

		set := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "values")
		for _, v := range values {
			set.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, v, "value"))
		}

		attr.AppendChild(set)
		attrList.AppendChild(attr)
	}

	entry.AppendChild(attrList)
	m.send(conn, msgID, entry)

	done := ber.Encode(ber.ClassApplication, ber.TypeConstructed, appSearchResultDone, nil, "Search Result Done")
	done.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, int64(ldapResultSuccess), "resultCode"))
	done.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matchedDN"))
	done.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnosticMessage"))

	m.send(conn, msgID, done)
}

// send wraps op in an LDAPMessage envelope and writes it.
func (m *mockLDAP) send(conn net.Conn, msgID int64, op *ber.Packet) {
	msg := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP Message")
	msg.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, msgID, "messageID"))
	msg.AppendChild(op)

	_, err := conn.Write(msg.Bytes())
	if err != nil {
		m.t.Logf("mock LDAP write: %v", err)
	}
}
