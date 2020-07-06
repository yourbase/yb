package remote

import (
	"context"
	"time"

	//"github.com/gorilla/websocket"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/yourbase/yb/plumbing/log"
	. "github.com/yourbase/yb/types"
)

func (r *RemoteBuild) Do(ctx context.Context) error {

}

func (cmd *RemoteBuild) submitBuild(project *Project, tagMap map[string]string) error {

	startTime := time.Now()
	var submitProgress, remoteProgress *Progress
	submitErrored := func() {
		if submitProgress != nil {
			submitProgress.Fail()
		}
	}
	remoteErrored := func() {
		if remoteProgress != nil {
			remoteProgress.Fail()
		}
	}

	/*
			(zombiezen) added:

		This goroutine worries me for a few reasons:

		    The goroutine will leak past the end of submitBuild if an interrupt signal is not sent to the process.
		    The calls to submitErrored and remoteErrored both race on the assignment to these local variables.
		    It races with indicating success: imagine the case where the build succeeds and then an interrupt signal is sent before the main goroutine is able to report success. It's undefined which one of these would be called first.

		I recommend that we start plumbing Context throughout the yb codebase, which is the standard pattern in Go for dealing with these sorts of issues. See https://talks.golang.org/2014/gotham-context.slide for a brief overview.

	*/
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		submitErrored()
		remoteErrored()
		os.Exit(1)
	}()

	if log.CheckIfTerminal() {
		submitProgress = NewProgressSpinner("Submitting remote build")
		submitProgress.Start()
	}

	userToken, err := ybconfig.UserToken()
	if err != nil {
		submitErrored()
		return err
	}

	patchBuffer := bytes.NewBuffer(cmd.patchData)

	if err = CompressBuffer(patchBuffer); err != nil {
		submitErrored()
		return fmt.Errorf("Couldn't compress the patch file: %s", err)
	}

	patchEncoded := base64.StdEncoding.EncodeToString(patchBuffer.Bytes())

	formData := url.Values{
		"project_id": {strconv.Itoa(project.Id)},
		"repository": {project.Repository},
		"api_key":    {userToken},
		"target":     {cmd.target},
		"patch_data": {patchEncoded},
		"commit":     {cmd.baseCommit},
		"branch":     {cmd.branch},
	}

	tags := make([]string, 0)
	for k, v := range tagMap {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	for _, tag := range tags {
		formData.Add("tags[]", tag)
	}

	if cmd.noAcceleration {
		formData.Add("no-accel", "True")
	}

	if cmd.disableCache {
		formData.Add("disable-cache", "True")
	}

	if cmd.disableSkipper {
		formData.Add("disable-skipper", "True")
	}

	resp, err := postToApi("builds/cli", formData)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		submitErrored()
		return fmt.Errorf("Couldn't read response body: %s", err)
	}
	switch resp.StatusCode {
	case 401:
		submitErrored()
		return fmt.Errorf("Unauthorized, authentication failed.\nPlease `yb login` again.")
	case 403:
		submitErrored()
		if cmd.publicRepo {
			return fmt.Errorf("This should not happen, please open a support inquery with YB")
		} else {
			return fmt.Errorf("Tried to build a private repository of a organization of which you're not part of.")
		}
	case 412:
		// TODO Show helpfull message with App URL to fix GH App installation issue
		submitErrored()
		return fmt.Errorf("Please verify if this specific repo has %s installed", ybconfig.CurrentGHAppUrl())
	case 500:
		submitErrored()
		return fmt.Errorf("Internal server error")
	}

	response := string(body)

	//Process simple response from the API
	response = strings.ReplaceAll(response, "\"", "")

	url := ""
	if strings.Count(response, "\n") > 0 {
		url = strings.Split(response, "\n")[0]
	} else {
		url = response
	}

	if submitProgress != nil {
		submitProgress.Success()
	}
	endTime := time.Now()
	submitTime := endTime.Sub(startTime)
	log.Infof("Submission finished at %s, taking %s", endTime.Format(TIME_FORMAT), submitTime.Truncate(time.Millisecond))

	startTime = time.Now()
	if log.CheckIfTerminal() {
		remoteProgress = NewProgressSpinner("Setting up remote build")
		remoteProgress.Start()
	}

	// Address fdbk: https://github.com/yourbase/yb/pull/90
	/*
		Resume of (zombiezen) ideas:
	*/

	if strings.HasPrefix(url, "ws:") || strings.HasPrefix(url, "wss:") {
		recentReconnect := false
		reconnectCount := 0
	CONN:
		conn, _, _, err := ws.DefaultDialer.Dial(context.Background(), url)

		finish := make(chan struct{})

		if err != nil {
			remoteErrored()
			return fmt.Errorf("Cannot connect: %v", err)
		} else {

			// TODO maybe make Dispatcher give better diagnostics somehow
			// TODO (zombiezen): Using time.After leaks a timer when <-finish is finally received from. Also, since this is a repeating edge, prefer using *time.Ticker:
			go func() {
				for {
					select {
					case <-finish:
						return
					case <-time.After(5 * time.Second):
						if err := wsutil.WriteClientMessage(conn, ws.OpPing, []byte("remotebuild ping")); err != nil {
							log.Errorf("Cannot send ping: %v", err)
						}
					}
				}
			}()

			defer func() {
				if err = conn.Close(); err != nil {
					log.Debugf("Cannot close: %v", err)
				}
			}()

			buildSuccess := false
			buildFailed := false
			buildSetupFinished := false
			for {
				msg, control, err := wsutil.ReadServerData(conn)
				if err != nil {
					// TODO (zombiezen): In code that targets Go 1.13 or above, prefer using errors.Is:
					if err == io.EOF {
						if buildSuccess {
							log.Infoln("Build Completed!")
							close(finish)
							return nil
						} else if buildFailed {
							log.Errorf("Build failed!")
							log.Infof("Build Log: %v", managementLogUrl(url, project.OrgSlug, project.Label))

							close(finish)
							return nil
						} else {
							if !recentReconnect && reconnectCount < 15 {
								if remoteProgress != nil {
									fmt.Println()
								}
								log.Tracef("Build not completed, trying to reconnect")
								conn.Close()
								close(finish)
								recentReconnect = true
								reconnectCount += 1
								// TODO (zombiezen): This nesting and goto makes this logic hard to follow. I recommend trying to factor out some of this into a separate function.
								goto CONN
							} else {
								if !buildSetupFinished {
									remoteErrored()
									log.Errorf("Patch failed, did you 'git rebase' recently?")
								} else {
									remoteErrored()
									log.Errorf("Unable to determine build status please check:")
								}
								log.Infof("Build Log: %v", managementLogUrl(url, project.OrgSlug, project.Label))

								close(finish)
								return nil
							}
						}
					}
					if err != io.EOF {
						log.Tracef("Unstable connection: %v", err)
					}
				} else {
					// TODO This depends on build agent output, try to structure this better
					if control.IsData() && strings.Count(string(msg), "Streaming results from build") > 0 {
						fmt.Println()
					} else if control.IsData() && !buildSetupFinished && len(msg) > 0 {
						buildSetupFinished = true
						if remoteProgress != nil {
							remoteProgress.Success()
						}
						endTime := time.Now()
						setupTime := endTime.Sub(startTime)
						log.Infof("Set up finished at %s, taking %s", endTime.Format(TIME_FORMAT), setupTime.Truncate(time.Millisecond))
						if cmd.publicRepo {
							log.Infof("Building a public repository: '%s'", project.Repository)
						}
						log.Infof("Build Log: %v", managementLogUrl(url, project.OrgSlug, project.Label))
					}
					if !buildSuccess {
						buildSuccess = strings.Count(string(msg), "-- BUILD SUCCEEDED --") > 0
					}
					if !buildFailed {
						buildFailed = strings.Count(string(msg), "-- BUILD FAILED --") > 0
						if !buildFailed {
							buildFailed = strings.Count(string(msg), "Patch '' didn't apply cleanly") > 0
						}
					}

					fmt.Printf("%s", msg)
					recentReconnect = false
				}
			}
		}
	} else {
		return fmt.Errorf("Unable to stream build output!")
	}

}
