package handlers

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/gif" // register GIF decoder so image.Decode handles GIFs

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// avatarMaxBytes is the maximum accepted size for a fetched avatar
	// image (5 MB).
	avatarMaxBytes = 5 << 20
	// avatarFetchTimeout is the maximum time allowed to download an avatar.
	avatarFetchTimeout = 10 * time.Second
	// avatarDialTimeout is the maximum time the SSRF-safe dialer waits
	// for a TCP connection.
	avatarDialTimeout = 5 * time.Second
	// avatarSubDir is the subdirectory of UploadsDir where avatars are stored.
	avatarSubDir = "avatars"
	// avatarJPEGQuality is the JPEG quality used when re-encoding avatars.
	avatarJPEGQuality = 90
	// avatarMaxRedirects limits how many HTTP redirects the fetch client follows.
	avatarMaxRedirects = 3
	// avatarDirPerm is the permission bits for the avatars directory.
	avatarDirPerm = 0o750
	// avatarFormatPNG is the format name returned by [image.Decode] for PNGs.
	avatarFormatPNG = "png"
)

// privateRanges lists all CIDR blocks that must not be reachable via a
// server-side fetch, to prevent SSRF. [net.IP.IsPrivate] covers RFC 1918
// and loopback but omits link-local (169.254/16) and shared-address
// (100.64/10), so those are included explicitly here.
//
//nolint:gochecknoglobals // computed once at init from fixed literals; equivalent to a const
var privateRanges = buildPrivateRanges()

// buildPrivateRanges parses the fixed set of private/reserved CIDR blocks
// into [net.IPNet] values ready for Contains checks.
func buildPrivateRanges() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / cloud metadata (AWS/GCP/Azure)
		"100.64.0.0/10",  // shared address space (RFC 6598)
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}

	nets := make([]*net.IPNet, 0, len(cidrs))

	for _, cidr := range cidrs {
		_, ipNet, _ := net.ParseCIDR(cidr)
		nets = append(nets, ipNet)
	}

	return nets
}

// isPrivateIP reports whether ip falls inside any private or reserved range.
func isPrivateIP(ip net.IP) bool {
	for _, block := range privateRanges {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// safeDial resolves addr, verifies that every resolved IP is outside all
// private/reserved ranges, then dials. Used as DialContext on the avatar HTTP
// client to block SSRF at the TCP layer — even when a redirect chain leads to
// a private host, the dial for that hop will be refused.
func safeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("split host/port: %w", err)
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}

	for _, a := range addrs {
		if isPrivateIP(a.IP) {
			return nil, fmt.Errorf("refusing to connect to private/reserved address %s", a.IP)
		}
	}

	dialer := &net.Dialer{Timeout: avatarDialTimeout}

	conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	return conn, nil
}

// avatarHTTPClient is the dedicated HTTP client for avatar fetches. It uses
// safeDial as its transport so SSRF checks apply on every TCP connection,
// including after redirects.
//
//nolint:gochecknoglobals // package-level client is intentional: reuses TCP connections across requests
var avatarHTTPClient = &http.Client{
	Timeout: avatarFetchTimeout,
	Transport: &http.Transport{
		DialContext: safeDial,
	},
	CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= avatarMaxRedirects {
			return errors.New("too many redirects")
		}

		// safeDial re-checks the resolved IP on each hop automatically.
		return nil
	},
}

// ServeAvatar serves a stored avatar file from the uploads/avatars directory.
func (s *State) ServeAvatar(writer http.ResponseWriter, request *http.Request) {
	filename := chi.URLParam(request, "filename")

	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		s.Error(writer, http.StatusBadRequest, "Invalid filename")

		return
	}

	path := filepath.Join(s.Config.UploadsDir, avatarSubDir, filename)
	http.ServeFile(writer, request, path) //nolint:gosec // filename validated above: no ".." or "/" allowed
}

// FetchAvatar godoc
// @Summary   Fetch and store an avatar image from an external URL
// @Tags      Auth
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "url (required)"
// @Success   200   {object}  userPublicRow
// @Failure   400   {object}  map[string]string
// @Router    /api/auth/avatar/fetch [post].
func (s *State) FetchAvatar(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		URL string `json:"url"`
	}

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	rawURL := strings.TrimSpace(body.URL)
	if rawURL == "" {
		s.Error(writer, http.StatusBadRequest, "url is required")

		return
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		s.Error(writer, http.StatusBadRequest, "url must be a valid http or https URL")

		return
	}

	img, format, fetchErr := fetchImage(request.Context(), rawURL)
	if fetchErr != nil {
		zap.L().Warn("avatar fetch failed", zap.String("url", rawURL), zap.Error(fetchErr))
		s.Error(writer, http.StatusBadRequest, fetchErr.Error())

		return
	}

	claims := s.claims(request)

	internalURL, storeErr := storeAvatar(s.Config.UploadsDir, claims.Subject, img, format)
	if storeErr != nil {
		zap.L().Error("avatar store failed", zap.Error(storeErr))
		s.Error(writer, http.StatusInternalServerError, "Failed to store avatar")

		return
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	user, err := s.Repos.Users.UpdateProfile(request.Context(), userID, nil, nil, &internalURL)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	s.JSON(writer, http.StatusOK, toUserPublicRow(user))
}

// fetchImage downloads the image at rawURL, enforces size and content-type
// limits, and decodes it into an [image.Image] ready for re-encoding.
// The second return value is the format name ("jpeg", "png", "gif", …).
func fetchImage(ctx context.Context, rawURL string) (image.Image, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, "", errors.New("invalid URL")
	}

	resp, err := avatarHTTPClient.Do(req)
	if err != nil {
		return nil, "", errors.New("could not fetch image from URL")
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("remote server returned status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return nil, "", errors.New("URL does not point to an image")
	}

	limited := io.LimitReader(resp.Body, avatarMaxBytes+1)

	img, format, err := image.Decode(limited)
	if err != nil {
		return nil, "", errors.New("could not decode image")
	}

	return img, format, nil
}

// UploadAvatar godoc
// @Summary   Upload a local image file as profile avatar
// @Tags      Auth
// @Security  BearerAuth
// @Accept    multipart/form-data
// @Produce   json
// @Param     avatar  formData  file  true  "Image file (max 5 MB)"
// @Success   200  {object}  userPublicRow
// @Failure   400  {object}  map[string]string
// @Router    /api/auth/avatar/upload [post].
func (s *State) UploadAvatar(writer http.ResponseWriter, request *http.Request) {
	//nolint:gosec // memory is bounded by the avatarMaxBytes constant (5 MB)
	err := request.ParseMultipartForm(avatarMaxBytes)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "File too large or invalid form")

		return
	}

	file, _, err := request.FormFile("avatar")
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "avatar field is required")

		return
	}

	defer func() { _ = file.Close() }()

	limited := io.LimitReader(file, avatarMaxBytes+1)

	img, format, err := image.Decode(limited)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "could not decode image")

		return
	}

	claims := s.claims(request)

	internalURL, storeErr := storeAvatar(s.Config.UploadsDir, claims.Subject, img, format)
	if storeErr != nil {
		zap.L().Error("avatar upload store failed", zap.Error(storeErr))
		s.Error(writer, http.StatusInternalServerError, "Failed to store avatar")

		return
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	user, err := s.Repos.Users.UpdateProfile(request.Context(), userID, nil, nil, &internalURL)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	s.JSON(writer, http.StatusOK, toUserPublicRow(user))
}

// storeAvatar re-encodes img (stripping EXIF and any embedded payload in the
// process) and writes it to uploadsDir/avatars/<userID>.<ext>. It returns the
// internal serving path /uploads/avatars/<filename>.
func storeAvatar(uploadsDir, userID string, img image.Image, format string) (string, error) {
	ext := ".jpg"
	if format == avatarFormatPNG {
		ext = ".png"
	}

	filename := userID + ext
	avatarDir := filepath.Join(uploadsDir, avatarSubDir)

	err := os.MkdirAll(avatarDir, avatarDirPerm)
	if err != nil {
		return "", fmt.Errorf("create avatar dir: %w", err)
	}

	//nolint:gosec // dest is built from a UUID (userID) and a fixed subdir — no path traversal possible
	file, err := os.Create(filepath.Join(avatarDir, filename))
	if err != nil {
		return "", fmt.Errorf("create avatar file: %w", err)
	}

	defer func() { _ = file.Close() }()

	if format == avatarFormatPNG {
		err = png.Encode(file, img)
	} else {
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: avatarJPEGQuality})
	}

	if err != nil {
		return "", fmt.Errorf("encode avatar: %w", err)
	}

	return "/uploads/avatars/" + filename, nil
}
