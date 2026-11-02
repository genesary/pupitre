package models

// GroupRoleMapping maps an IdP/local group name to the platform role its
// members should be granted ("admin", "manager" or "student").
type GroupRoleMapping struct {
	GroupName    string `gorm:"column:groupname;size:255;primaryKey"`
	PlatformRole string `gorm:"column:platformrole;size:16;not null;check:platformrole IN ('admin','manager','student')"`
}

// TableName pins GroupRoleMapping to the group_role_mappings table.
func (GroupRoleMapping) TableName() string { return "group_role_mappings" }
