package types

import (
	"testing"
)

func TestGitRemoteParse(t *testing.T) {

	for _, l := range []struct {
		in       string
		token    string
		user     string
		password string
		valid    bool
		rType    RemoteType
	}{
		{
			in:       "https://x-access-token:X86jsl@github.com/SpankChain/spankpay.git",
			token:    "X86jsl",
			user:     "x-access-token",
			password: "X86jsl",
			valid:    true,
			rType:    HttpsRemote,
		},
		{
			in:       "https://x-access-token:token:X86jsl@github.com/beholders-eye/godif.git",
			token:    "X86jsl",
			user:     "x-access-token",
			password: "token",
			valid:    true,
			rType:    HttpsRemote,
		},
		{
			in:       "https://x-access-token:token:@github.com/something/t",
			token:    "",
			user:     "x-access-token",
			password: "token",
			valid:    true,
			rType:    HttpsRemote,
		},
		{
			in:    "http://x-access-Token:token:gooehjaoo heo  goej oag eo @github.com/SpankChain/spankpay.git",
			valid: false,
			rType: HttpRemote,
		},
		{
			in:       "http://x-access-token:token:gooehjaoo heo  goej oag eo @github.com/SpankChain/spankpay.git",
			token:    "gooehjaoo heo  goej oag eo ",
			user:     "x-access-token",
			password: "token",
			valid:    true,
			rType:    HttpRemote,
		},
		{
			in:    "sbrubbles:/x-access-token:token:gooehjaoo heo  goej oag eo @github.com/SpankChain/spankpay.git",
			valid: false,
		},
		{
			in:    "sbrubbles://x-access-token:token:gooehjaoo heo  goej oag eo @github.com/SpankChain/spankpay.git",
			valid: false,
		},
		{
			in:    "git:x-access-token:token:ahejoheoheoehjoe@github.com:checkit/git.git",
			valid: false,
		},
		{
			in:    "https://x-access-token:token:X86jsl:gjoegeo:ajgoe888:l@github.com/beholders-eye/godif.git",
			valid: false,
			rType: HttpsRemote,
		},
		{
			in:       "git@github.com:yourbase/ybdocs",
			token:    "",
			user:     "git",
			password: "",
			valid:    true,
			rType:    SshRemote,
		},
		{
			in:       "git:calhamba@gitlab.xu:something/where",
			token:    "",
			user:     "git",
			password: "calhamba",
			valid:    true,
			rType:    SshRemote,
		},
		{
			in:       "ssh://git@gitlab.xu:something/where",
			token:    "",
			user:     "git",
			password: "",
			valid:    true,
			rType:    SshRemote,
		},
	} {
		got := NewGitRemote(l.in)
		t.Logf("Remote '%s'...", l.in)
		if v := got.Validate(); v != l.valid {
			t.Errorf("Validation: got: '%v' wanted: '%v'", v, l.valid)
		}

		if l.token != got.Token {
			t.Errorf("Token: got: '%s' wanted: '%s'", got.Token, l.token)
		}

		if l.user != got.User {
			t.Errorf("User: got: '%s' wanted: '%s'", got.User, l.user)
		}

		if l.password != got.Password {
			t.Errorf("Password: got: '%s' wanted: '%s'", got.Password, l.password)
		}

		if l.rType != got.Type {
			t.Errorf("Password: got: '%v' wanted: '%v'", got.Type, l.rType)
		}
	}
}
