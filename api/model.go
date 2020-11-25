package api

//TaskView est le json pour demaander l'execution d'une tache
type TaskView struct {
	Type    string `json:"type"`      //type de tache
	Timeout int64  `json:"timeout"`   //délai d'exec max en millisecondes
	LogCfg  string `json:"log_store"` //config log à appliquer

	Cmd     string   `json:"cmd"`      //Tache type commande : - path appli a exec
	Args    []string `json:"args"`     // - et ses args
	StartIn string   `json:"start_in"` // - dossier de démarage

	URL string `json:"url"` //Tache type check url up : - url à  controler
}

//TaskReponse est le résultat de l'execution
type TaskReponse struct {
	ID         int64  `json:"id"`                    //id tache
	OnRegister bool   `json:"on_register"`           //id de tache dont le résultat est encore connue (faux peut indiquer soit que l'id n'a jamais existé, soit qu'il a traité mais on n'a plus son résultat à dispo)
	Terminated bool   `json:"terminated"`            //tache connue comme terminé
	ResOK      bool   `json:"result"`                //résultat (ok ou ko)
	ResInfo    string `json:"result_info,omitempty"` //info resultat
	Duration   int64  `json:"duration"`              //durée d'execution en ms
}
