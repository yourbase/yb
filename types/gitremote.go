package types

import (
	"fmt"
	"strings"
)

type RemoteType int

const (
	SshRemote RemoteType = iota
	HttpsRemote
	HttpRemote
	GitRawRemote
	FileRemote
)

type GitRemote struct {
	Url      string
	Protocol string
	Token    string
	User     string
	Password string
	Domain   string
	Path     string
	Branch   string
	Type     RemoteType
}

func NewGitRemote(url string) (r GitRemote) {
	r.Url = url
	r.Token, r.User, r.Password, r.Domain, r.Path, r.Protocol, r.Type = parseRemote(url)

	return
}

func (r GitRemote) Validate() bool {
	return r.Url != "" && r.Domain != "" && r.Path != ""
}

func (r GitRemote) String() (str string) {
	if r.Validate() {
		var template, auth string
		n := 3

		switch {
		case r.Type == SshRemote || r.Type == GitRawRemote:
			template = "%s%s:%s" // credentials@host:path
			if r.User != "" {
				auth = fmt.Sprintf("%s@", r.User)
				if r.Password != "" {
					auth = fmt.Sprintf("%s:%s@", r.User, r.Password)
				}
			}
		case r.Type == FileRemote:
			n = 2
			template = "%s:///%s" // file:///path
		case r.Type == HttpsRemote || r.Type == HttpRemote:
			n = 4
			template = "%s://%s%s/%s" // prot://credentials@domain/path
			if r.Token != "" {
				auth = fmt.Sprintf("x-access-token:%s@", r.Token)
			} else if r.User != "" {
				auth = fmt.Sprintf("%s:%s@", r.User, r.Password)
			}
		}
		switch n {
		case 2:
			str = fmt.Sprintf(template, r.Protocol, r.Path)
		case 3:
			str = fmt.Sprintf(template, auth, r.Domain, r.Path)
		case 4:
			str = fmt.Sprintf(template, r.Protocol, auth, r.Domain, r.Path)
		}
	}
	return
}

func parseRemote(s string) (token, user, password, domain, path, protocol string, t RemoteType) {
	var auth, afterAuth string

	clean := func() {
		token = ""
		user = ""
		password = ""
		domain = ""
		path = ""
		protocol = ""
	}

	setType := func(prot string) bool {
		protocol = strings.ToLower(prot)
		r := true
		switch prot {
		case "http":
			t = HttpRemote
		case "https":
			t = HttpsRemote
		case "ssh":
			t = SshRemote
		case "git":
			t = GitRawRemote
		case "file":
			t = FileRemote
		default:
			r = false
		}
		return r
	}

	parts := strings.Split(s, "@")
	if len(parts) > 1 {
		protocolAndAuth := strings.Split(parts[0], "://")
		if len(protocolAndAuth) == 2 {
			if !setType(protocolAndAuth[0]) {
				// clean all, invalid protocol
				clean()
				return
			}
			auth = protocolAndAuth[1]
		} else {
			auth = parts[0]
		}

		if strings.Contains(auth, ":/") {
			clean()
			return
		}

		if strings.Contains(auth, ":") {
			creds := strings.Split(auth, ":")
			user = creds[0]
			password = creds[1]
			if (t == HttpsRemote || t == HttpRemote) && user == "x-access-token" {
				if len(creds) > 2 && creds[1] == "token" {
					// push remotes has a ":token:" piece
					token = creds[2]
					if len(creds) > 3 {
						// Malformed/bogus
						token = ""
						user = ""
						password = ""
					}
				} else {
					token = password
				}
			} else {
				if len(creds) > 2 {
					// Malformed password (a lot of ":")
					clean()
					return
				}
			}
		} else {
			user = auth
		}
		if user != "" {
			afterAuth = parts[1]
		}
	} else {
		protocolAndLoc := strings.Split(s, "://")
		if len(protocolAndLoc) > 1 {
			if !setType(protocolAndLoc[0]) {
				// clean all, invalid protocol
				clean()
				return
			}
			afterAuth = protocolAndLoc[1]
		}
	}
	if t != SshRemote && t != GitRawRemote {
		remainder := strings.Split(afterAuth, "/")
		if len(remainder) > 1 {
			domain = remainder[0]
			path = strings.Join(remainder[1:], "/")
		}
	} else {
		sides := strings.Split(afterAuth, ":")
		if len(sides) > 1 {
			domain = sides[0]
			path = sides[1]
		}
	}

	return
}
