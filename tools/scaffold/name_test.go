package scaffold

import "testing"

func TestToSnake(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"Foo", "foo"},
		{"FooBar", "foo_bar"},
		{"OrgMembership", "org_membership"},
		{"HTTPServer", "http_server"},
		{"UserID", "user_id"},
		{"A", "a"},
		{"AB", "ab"},
		{"ABC", "abc"},
		{"ABCDef", "abc_def"},
	}
	for _, c := range cases {
		got := ToSnake(c.in)
		if got != c.want {
			t.Errorf("ToSnake(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestToLowerCamel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"Foo", "foo"},
		{"FooBar", "fooBar"},
		{"Login", "login"},
		{"SelectOrg", "selectOrg"},
		{"ListUserOrgs", "listUserOrgs"},
		{"A", "a"},
	}
	for _, c := range cases {
		got := ToLowerCamel(c.in)
		if got != c.want {
			t.Errorf("ToLowerCamel(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestIsValidExportedIdent(t *testing.T) {
	valid := []string{"Foo", "FooBar", "F", "F1", "Foo_bar"}
	invalid := []string{"", "foo", "1Foo", "foo bar", "Foo-Bar", "_Foo"}

	for _, s := range valid {
		if !IsValidExportedIdent(s) {
			t.Errorf("IsValidExportedIdent(%q) = false; want true", s)
		}
	}
	for _, s := range invalid {
		if IsValidExportedIdent(s) {
			t.Errorf("IsValidExportedIdent(%q) = true; want false", s)
		}
	}
}
