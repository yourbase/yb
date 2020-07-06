package remote

func postJsonToApi(path string, jsonData []byte) (*http.Response, error) {
	userToken, err := ybconfig.UserToken()

	if err != nil {
		return nil, err
	}

	apiUrl, err := ybconfig.ApiUrl(path)

	if err != nil {
		return nil, fmt.Errorf("Unable to generate API URL: %v", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	return res, err

}

func postToApi(path string, formData url.Values) (*http.Response, error) {
	userToken, err := ybconfig.UserToken()

	if err != nil {
		return nil, fmt.Errorf("Couldn't get user token: %v", err)
	}

	apiUrl, err := ybconfig.ApiUrl(path)
	if err != nil {
		return nil, fmt.Errorf("Couldn't determine API URL: %v", err)
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("Couldn't make API call: %v", err)
	}

	req.Header.Set("YB_API_TOKEN", userToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func managementLogUrl(url, org, label string) string {
	wsUrlRegexp := regexp.MustCompile(`^wss?://[^/]+/builds/([0-9a-f-]+)/progress$`)

	if wsUrlRegexp.MatchString(url) {
		submatches := wsUrlRegexp.FindStringSubmatch(url)
		build := ""
		if len(submatches) > 1 {
			build = submatches[1]
		}
		if len(build) == 0 {
			return ""
		}

		u, err := ybconfig.ManagementUrl(fmt.Sprintf("/%s/%s/builds/%s", org, label, build))
		if err != nil {
			log.Errorf("Unable to generate App Url: %v", err)
		}

		return u
	}
	return ""
}

func (p *RemoteBuild) fetchProject(urls []string) (*Project, GitRemote, error) {
	var empty GitRemote
	v := url.Values{}
	fmt.Println()
	log.Infof("URLs used to search: %s", urls)

	for _, u := range urls {
		rem := NewGitRemote(u)
		// We only support GitHub by now
		// TODO create something more generic
		if rem.Validate() && strings.Contains(rem.String(), "github.com") {
			p.remotes = append(p.remotes, rem)
			v.Add("urls[]", u)
		} else {
			log.Warnf("Invalid remote: '%s', ignoring", u)
		}
	}
	resp, err := postToApi("search/projects", v)

	if err != nil {
		return nil, empty, fmt.Errorf("Couldn't lookup project on api server: %v", err)
	}

	if resp.StatusCode != 200 {
		log.Debugf("Build server returned HTTP Status %d", resp.StatusCode)
		if resp.StatusCode == 203 {
			p.publicRepo = true
		} else if resp.StatusCode == 401 {
			return nil, empty, fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
		} else if resp.StatusCode == 412 || resp.StatusCode == 404 {
			return nil, empty, fmt.Errorf("Please verify if this private repository has %s installed.", ybconfig.CurrentGHAppUrl())
		} else {
			return nil, empty, fmt.Errorf("This is us, not you, please try again in a few minutes.")
		}
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, empty, err
	}
	var project Project
	err = json.Unmarshal(body, &project)
	if err != nil {
		return nil, empty, err
	}

	remote := p.pickRemote(project.Repository)
	if !remote.Validate() {
		return nil, empty, fmt.Errorf("Can't pick a good remote to clone upstream.")
	}

	return &project, remote, nil
}
