package task

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ensure we always implement Tasker
var _ Tasker = (*CmdTask)(nil)

//CmdTask tache type commande
type CmdTask struct {
	Cmd     string        //Commande principale
	Args    []string      //et ses args
	Timeout time.Duration //tps d'exec max
	StartIn string        //dossier de démarage
}

//CheckValid retourne une erreur si la tache semble irrealisable
func (c CmdTask) CheckValid() error {
	if c.Cmd == "" {
		return fmt.Errorf("cmd emty")
	}
	//check présence app
	_, err := exec.LookPath(c.Cmd)
	if err != nil {
		return fmt.Errorf("%v not fourd", c.Cmd)
	}
	return nil
}

//Run lance la tache
func (c CmdTask) Run(logto *log.Logger) (bool, string) {
	//prepa commande avec sans timeout
	var (
		cmd    *exec.Cmd
		ctx    context.Context
		cancel context.CancelFunc
		bOk    bool = false
		rInfo  string
	)
	if c.Timeout <= 0 {
		cmd = exec.Command(c.Cmd, c.Args...)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), c.Timeout)
		defer cancel()

		cmd = exec.CommandContext(ctx, c.Cmd, c.Args...)
	}
	if c.StartIn != "" {
		cmd.Dir = c.StartIn
	}

	//les remonté (stdout et err) de l'exe sont tracé dans le log assigné
	cmd.Stdout = logto.Writer()
	cmd.Stderr = logto.Writer()

	//appel bloquant :
	dtStart := time.Now()
	logto.Println("Start", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	//recup retours de l'appels
	if ctx != nil && ctx.Err() == context.DeadlineExceeded {
		rInfo = "timeout"
		logto.Println("TIMEOUT (", c.Timeout, ")", strings.Join(cmd.Args, " "), ", duration=", time.Since(dtStart))
	} else if _, ok := err.(*exec.ExitError); ok {
		//erreur coté appli (retour diff de zero)
		rInfo = "exit code = " + strconv.Itoa(cmd.ProcessState.ExitCode())
		logto.Println("Terminated with error", cmd.ProcessState.ExitCode(), strings.Join(cmd.Args, " "), ", duration=", time.Since(dtStart))
	} else if err != nil {
		rInfo = err.Error()
		logto.Println("Command Error", strings.Join(cmd.Args, " "), ", err=", err)
	} else {
		bOk = true
	}
	if bOk {
		logto.Println("Terminated", strings.Join(cmd.Args, " "), ", duration=", time.Since(dtStart))
	}

	return bOk, rInfo
}
