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

func TestIsValidPackageIdent(t *testing.T) {
	valid := []string{"http", "cli", "mcp", "grpc", "redis_queue", "a", "a1", "foo_bar_baz"}
	invalid := []string{
		"", "Http", "HTTP", "1grpc", "_cli", "redis-queue", "redis queue",
		"redisQueue",
		"func", "type", "package", "interface", // keywords
	}
	for _, s := range valid {
		if !IsValidPackageIdent(s) {
			t.Errorf("IsValidPackageIdent(%q) = false; want true", s)
		}
	}
	for _, s := range invalid {
		if IsValidPackageIdent(s) {
			t.Errorf("IsValidPackageIdent(%q) = true; want false", s)
		}
	}
}
