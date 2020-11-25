package task

import (
	"log"
	"net/http"
	"net/url"
	"time"
)

// ensure we always implement Tasker
var _ Tasker = (*URLCheckTask)(nil)

//URLCheckTask tache type test url http up
type URLCheckTask struct {
	URL     string        //url à tester http[s]:[port]//host/path
	Timeout time.Duration //tps d'exec max
}

//CheckValid retourne une erreur si la tache semble irrealisable
func (c URLCheckTask) CheckValid() error {
	_, err := url.ParseRequestURI(c.URL)
	return err
}

//Run lance la tache
func (c URLCheckTask) Run(logto *log.Logger) (bool, string) {
	var (
		bOk   bool = false
		rInfo string
	)

	// appli timeout et autre config
	client := http.Client{}
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}

	dtStart := time.Now()
	logto.Println("GET", c.URL)
	resp, err := client.Get(c.URL)
	if err != nil {
		rInfo = err.Error()
		logto.Println("ERR GET", c.URL, ":", err, ", duration=", time.Since(dtStart))
	} else {
		rInfo = "Status = " + resp.Status
		if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
			bOk = true
		}
	}
	if bOk {
		logto.Println("Terminated", c.URL, ", duration=", time.Since(dtStart))
	}

	return bOk, rInfo
}
