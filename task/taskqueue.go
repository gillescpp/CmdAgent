package task

import (
	"CmdAgent/slog"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

// Queue est la todo liste globale
var Queue TQueue

// Constantes
const (
	queueFile        string        = "queue.json"     //fichier persitance queue en cours
	taskInfoLifeSpan time.Duration = 30 * time.Minute //durée de rentention d'info sur les taches traités
	taskTimeOut      time.Duration = 10 * time.Minute //délai max de traitement pour une tache new
)

// Statut d'un tache
// TaskNew : en attente de traitement
// TaskRunning : en cours
// TaskTerminated : terminé
// TaskAborted : aborté (reboot svc pendant l'exec)
const (
	TaskNew        string = "TASK_NEW"
	TaskRunning    string = "TASK_RUNNING"
	TaskTerminated string = "TASK_DONE"
	TaskAborted    string = "TASK_ABORTED"
)

//Tasker contrat pour un tache
type Tasker interface {
	Run(logto *log.Logger) (bool, string) //corp de la tache
	CheckValid() error                    //auto validation
}

//TTask est un tache
type TTask struct {
	ID             int64     `json:"id"`            //id tache
	Task           Tasker    `json:"-"`             //tache à executer
	TaskType       string    `json:"task_type"`     //typage tache
	TaskSerialised string    `json:"task"`          //json tache avec ses infos spécifique (persistance disque)
	Status         string    `json:"status"`        //status de la tache
	LastActivity   time.Time `json:"last_activity"` //date derniere news
	StartedAt      time.Time `json:"started_at"`    //heure de démarage
	TerminatedAt   time.Time `json:"terminated_at"` //heure de fin
	TaskResOk      bool      `json:"result_ok"`     //info retour tache
	TaskResInfo    string    `json:"result_info"`   //info retour tache
	LogCfg         string    `json:"log_config"`    //config trace à appliquer
}

// nomarshJSONTTask : ce type sert d'intermédire dans le marchal/unmarshal
//(car l'appel des méthode json.Marshal/Unmarshal appelle les méthodes MarshalJSON/UnmarshalJSON de la cible
// si celle-ci implemente l'interface Marshaler.)
type nomarshJSONTTask TTask

// MarshalJSON : (Marshaler interface) Traitement des champs soumis a marshaling spec
func (c *TTask) MarshalJSON() ([]byte, error) {
	//marshaling de la tache stockée sur TaskSerialised
	c.TaskSerialised = ""
	if c.Task != nil {
		buffer, err := json.Marshal(c.Task)
		if err != nil {
			return nil, err
		}
		c.TaskSerialised = string(buffer)
	}
	//Puis on lance le marshaling standard (cast en type intermédiaire pour eviter que json.Marshal rapelle cette même méthode)
	c2 := nomarshJSONTTask(*c)
	return json.Marshal(&c2)
}

// UnmarshalJSON : (Unmarshaler interface) Traitement des champs soumis a unmarshaling spec
func (c *TTask) UnmarshalJSON(data []byte) error {
	var err error
	//Deserialisation standard et on renseigne les champs de donnée basé sur un marshalling
	//spec au travers d'un champs *CustomMarshal
	var c2 nomarshJSONTTask
	if err = json.Unmarshal(data, &c2); err != nil {
		return err
	}

	//puis spec au type de la tache (selon type)
	if c2.TaskType == "CmdTask" {
		var t CmdTask
		err := json.Unmarshal([]byte(c2.TaskSerialised), &t)
		if err != nil {
			return fmt.Errorf("Unmarshal CmdTask : %w", err)
		}
		c2.Task = t
	} else if c2.TaskType == "URLCheckTask" {
		var t URLCheckTask
		err := json.Unmarshal([]byte(c2.TaskSerialised), &t)
		if err != nil {
			return fmt.Errorf("Unmarshal URLCheckTask : %w", err)
		}
		c2.Task = t
	} else {
		return fmt.Errorf("TaskType %v unknown", c2.TaskType)
	}

	*c = TTask(c2)
	return nil
}

// TypeIsValid retourne true si le type de tache fourni est connnu
func TypeIsValid(taskType string) bool {
	ok := false
	switch taskType {
	case "CmdTask":
		ok = true
	case "URLCheckTask":
		ok = true
	}
	return ok
}

//TQueue est la todo liste en cours
type TQueue struct {
	mu     sync.Mutex       `json:"-"`      //accés exclusif
	Cnt    int64            `json:"cnt"`    //id tache
	Qtasks map[int64]*TTask `json:"qtasks"` //tableau des taches
	qPath  string           `json:"-"`      //fichier persistance
}

//save persitance sur disque de la queue
func (c *TQueue) save() error {
	//purge queue
	c.purge(false)

	buffer, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return fmt.Errorf("MarshalIndent queue : %w", err)
	}
	err = ioutil.WriteFile(c.qPath, buffer, 0644)
	if err != nil {
		return fmt.Errorf("WriteFile queue : %w", err)
	}
	return nil
}

// Add ajout d'un tache à la queue
func (c *TQueue) Add(task Tasker, logCfg string) (int64, error) {
	if logCfg == "" {
		logCfg = "_default_"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.Cnt++
	c.Qtasks[c.Cnt] = &TTask{
		ID:           c.Cnt,
		Task:         task,
		Status:       TaskNew,
		LastActivity: time.Now(),
		TaskType:     reflect.TypeOf(task).Name(),
		LogCfg:       logCfg,
	}

	//persist
	err := c.save()
	if err != nil {
		delete(c.Qtasks, c.Cnt)
		c.Cnt--
		return 0, err
	}

	return c.Cnt, nil
}

// Init init queue persisté
func (c *TQueue) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	//queue à coté de l'exe
	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Exec path : %w", err)
	}
	exePath := filepath.Dir(ex)
	c.qPath = filepath.Join(exePath, queueFile)

	if _, err := os.Stat(c.qPath); os.IsNotExist(err) {
		//fichier à créer
		c.Cnt = 0
		c.Qtasks = make(map[int64]*TTask)

		return c.save()
	}

	//deserialise
	buffer, err := ioutil.ReadFile(c.qPath)
	if err != nil {
		return fmt.Errorf("ReadFile %s : %w", c.qPath, err)
	}
	err = json.Unmarshal(buffer, c)
	if err != nil {
		return fmt.Errorf("Unmarshal %s : %w", c.qPath, err)
	}

	//purge queue
	c.purge(true)

	return nil
}

// purge supprime les elements plus nécessaires
// les new sont conservé, tous le reste est conservé selon une limite de temps
func (c *TQueue) purge(start bool) {
	for k, v := range c.Qtasks {
		//au démarrage, les "running" sont considérés comme aborté
		if start && v.Status == TaskRunning {
			c.Qtasks[k].Status = TaskAborted
			c.Qtasks[k].LastActivity = time.Now()
		}
		//cas des news en attente depuis trop longtemps
		if v.Status == TaskNew && time.Since(c.Qtasks[k].LastActivity) > taskTimeOut {
			c.Qtasks[k].Status = TaskAborted
			c.Qtasks[k].LastActivity = time.Now()
		}

		//info tache conservé taskInfoLifeSpan
		if v.Status != TaskNew && v.Status != TaskRunning && time.Since(c.Qtasks[k].LastActivity) > taskInfoLifeSpan {
			delete(c.Qtasks, k)
		}
	}
}

// PopNext retourne et lance la prochaine tache a exec et la passe en running
func (c *TQueue) PopNext() (*TTask, error) {
	var rt *TTask
	c.mu.Lock()
	defer c.mu.Unlock()

	var next int64 = -1
	for k, v := range c.Qtasks {
		if v.Status == TaskNew {
			next = k
			break
		}
	}

	//passe en running
	if next > 0 {
		t := c.Qtasks[next]
		t.Status = TaskRunning
		t.LastActivity = time.Now()
		t.StartedAt = time.Now()

		//persist
		err := c.save()
		if err != nil {
			t.Status = TaskNew
			t.StartedAt = time.Time{}
			return rt, err
		}
		rt = t

		//launch
		slog.To("").Println("Launch Task", t.ID)
		go func() {
			slog.To(t.LogCfg).Println("-- TASK ", t.ID, " BEGIN --")

			bRes, sRes := t.Task.Run(slog.To(t.LogCfg))
			err := Queue.Terminated(t.ID, bRes, sRes)
			if err != nil {
				slog.To("").Println("Error Terminated :", err)
			}
			slog.To(t.LogCfg).Println("-- TASK", t.ID, "END --")

			slog.To("").Println("Task terminated", t.ID, "res:", bRes, "info:", sRes)
		}()
	}
	return rt, nil
}

// Terminated déclare une tache terminé
func (c *TQueue) Terminated(taskID int64, bRes bool, resInfo string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Qtasks[taskID]; exists {
		c.Qtasks[taskID].Status = TaskTerminated
		c.Qtasks[taskID].LastActivity = time.Now()
		c.Qtasks[taskID].TerminatedAt = time.Now()
		c.Qtasks[taskID].TaskResOk = bRes
		c.Qtasks[taskID].TaskResInfo = resInfo

		return c.save()
	}
	return nil
}

// Get retourne l'état d'un tache
func (c *TQueue) Get(taskID int64) TTask {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cp TTask
	if _, exists := c.Qtasks[taskID]; exists {
		cp = *(c.Qtasks[taskID])
	}
	return cp
}
