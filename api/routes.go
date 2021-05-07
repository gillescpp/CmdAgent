package api

import (
	"CmdAgent/slog"
	"CmdAgent/task"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

//APIKey api key requis pour l'api
var APIKey string

//PanicHandler cas des requetes qui léverait un panic (evite que le program crash)
func PanicHandler(w http.ResponseWriter, r *http.Request, err interface{}) {
	http.Error(w, fmt.Sprintf("Error %v", err), http.StatusInternalServerError)
	slog.To("").Println("PanicHandler :", err)
}

//Ping handler test connectivité
func Ping(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	//si clé api fournie (ping utilisé pour tester la validité de la clé), on la valide et répond "OK" si c'est le cas
	if CheckAutorisation(r) == nil {
		fmt.Fprintf(w, "OK")
		return
	}
	fmt.Fprintf(w, "pong")
}

// CheckAutorisation controle api key
func CheckAutorisation(r *http.Request) error {
	var err error = fmt.Errorf("invalid api key")
	if v, exists := r.Header["X-Api-Key"]; exists && len(v) >= 1 {
		if v[0] == APIKey {
			err = nil
		}
	}
	return err
}

// checkLogGrp controle la config de log demandé
func checkLogGrp(logCfg string) error {
	if strings.ContainsAny(logCfg, ".\\/~_&#") { //carateres interdit
		return fmt.Errorf("forbidden character")
	}
	grpOk := slog.GrpExists(logCfg)
	if !grpOk {
		return fmt.Errorf("not configured")
	}
	return nil
}

//TaskCreateAsync handler POST pour la création d'un tache async
func TaskCreateAsync(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// validation : autorisation
	err := CheckAutorisation(r)
	if err != nil {
		sendErrResp(w, err.Error(), http.StatusForbidden)
		slog.To("").Println("[API] Call rejected :", err)
		return
	}

	//deserialise commande en input
	var input TaskView
	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		sendErrResp(w, err.Error(), http.StatusBadRequest)
		slog.To("").Println("[API] Call rejected :", err)
		return
	}

	// ctrls input
	if !task.TypeIsValid(input.Type) {
		sendErrResp(w, fmt.Sprintf("invalid task type %v", input.Type), http.StatusBadRequest)
		slog.To("").Println("[API] Call rejected : invalid task", input.Type)
		return
	}
	if err = checkLogGrp(input.LogCfg); err != nil {
		sendErrResp(w, fmt.Sprintf("invalid log_store %v : %v", input.LogCfg, err), http.StatusBadRequest)
		slog.To("").Println("[API] Call rejected : invalid log_store", input.LogCfg)
		return
	}

	var newTask task.Tasker
	if input.Type == "CmdTask" {
		newTask = task.CmdTask{
			Cmd:     input.Cmd,
			Args:    input.Args,
			Timeout: time.Duration(input.Timeout) * time.Millisecond,
			StartIn: input.StartIn,
		}
	} else if input.Type == "URLCheckTask" {
		newTask = task.URLCheckTask{
			URL:     input.URL,
			Timeout: time.Duration(input.Timeout) * time.Millisecond,
		}
	}

	//test tache réalisable
	err = newTask.CheckValid()
	if err != nil {
		sendErrResp(w, fmt.Sprintf("task rejected : %v", err), http.StatusInternalServerError)
		slog.To("").Println("[API] Call rejected :", err)
		return
	}

	//ajout à la queue
	id, err := task.Queue.Add(newTask, input.LogCfg)
	if err != nil {
		sendErrResp(w, fmt.Sprintf("task queue error : %v", err), http.StatusInternalServerError)
		slog.To("").Println("[API] Append to queue fail :", err)
		return
	}

	//retour immédiat : 202 Accepted et url queue en Content-Location
	tsid := struct {
		ID int64 `json:"id"` //id tache
	}{
		ID: id,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Location", r.URL.Path+"/"+strconv.FormatInt(id, 10))
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(tsid)
	slog.To("").Println("[API] New task", id, "accepted")
}

//TaskGet handler GET état d'une tache
func TaskGet(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// validation : autorisation
	err := CheckAutorisation(r)
	if err != nil {
		sendErrResp(w, err.Error(), http.StatusForbidden)
		return
	}

	//id demandé
	strid := p.ByName("tid")
	tid, _ := strconv.ParseInt(strid, 10, 64)
	if tid <= 0 {
		sendErrResp(w, fmt.Sprintf("invalid parameter %v", strid), http.StatusBadRequest)
		return
	}

	tsk := task.Queue.Get(tid)
	tskResp := TaskReponse{
		ID:         tid,
		OnRegister: false, //pas connu jusqu'a preuve du contraire
		Terminated: false,
		ResOK:      false,
		ResInfo:    "",
		Duration:   0,
	}

	if tsk.ID > 0 {
		//report info queue
		tskResp.OnRegister = true
		tskResp.Terminated = (tsk.Status == task.TaskTerminated || tsk.Status == task.TaskAborted)
		tskResp.ResOK = tsk.TaskResOk
		tskResp.ResInfo = tsk.TaskResInfo
		if tskResp.Terminated {
			tskResp.Duration = tsk.TerminatedAt.Sub(tsk.StartedAt).Milliseconds() //terminé : tps d'exec en ms
		} else if tsk.Status == task.TaskRunning {
			tskResp.Duration = time.Since(tsk.StartedAt).Milliseconds() //en cours, tps d'exec jusqu'ici, en ms
		}
	}

	//test tache réalisable
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tskResp)

}

//sendErrResp retourne une erreur avec msg en json
func sendErrResp(w http.ResponseWriter, Msg string, code int) {
	msg := struct {
		Message string `json:"message"`
	}{
		Message: Msg,
	}

	//out json
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(msg)
}
