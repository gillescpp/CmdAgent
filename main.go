package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"CmdAgent/api"
	"CmdAgent/slog"
	"CmdAgent/task"

	"github.com/julienschmidt/httprouter"
)

func main() {
	//trace par defaut avant prise en compte config
	slog.InitLogs("", "", 0, 0, false)

	//chargement config
	slog.To("").Println("lecture fichier de config...")
	err := InitConfig()
	if err != nil {
		slog.To("").Fatalln("Echec lecture fichier de configuration", err)
	}

	//init logs principaux (sortie standard)
	fl := ""
	if Config.LogStdConfig.LogFolder != "" {
		fl = filepath.Join(Config.LogStdConfig.LogFolder, "CmdAgent.log")
	}
	slog.InitLogs("", fl, Config.LogStdConfig.MaxSizeMB, Config.LogStdConfig.MaxBackups, Config.LogStdConfig.Compress)

	//puis autre groupe de log dans le fichier de config
	for k, v := range Config.LogTaskConfig {
		fla := ""
		if v.LogFolder != "" {
			fla = filepath.Join(v.LogFolder, k+".log")
		}
		slog.InitLogs(k, fla, v.MaxSizeMB, v.MaxBackups, v.Compress)
	}

	//gen certificat auto signé (api en https imposé)
	cert, key, err := api.GenerateSelfSignedCert()
	if err != nil {
		slog.To("").Fatalln("Erreur certificat", err)
	}

	// init queue taches
	slog.To("").Println("Init task queue...")
	err = task.Queue.Init()
	if err != nil {
		slog.To("").Fatalf("Queue.Init %v", err)
	}

	//boucle de démarage des taches
	go func() {
		for {
			time.Sleep(200 * time.Millisecond) //note pourrait remplacer par un chan

			task, e := task.Queue.PopNext()
			if e != nil {
				slog.To("").Println("Pop Task error", task, e)
				time.Sleep(5000 * time.Second)
			}
		}
	}()

	//Mise en écoute de l'interface REST
	restPort := Config.ListenPort
	bNoTLS := Config.NoTLS
	strListenOn := ":" + strconv.Itoa(restPort)
	api.APIKey = Config.APIKey
	if Config.APIKey == "" {
		slog.To("").Fatalf("API key non définie")
	}
	if bNoTLS {
		slog.To("").Println("Ecoute sur http", strListenOn, "...")
	} else {
		slog.To("").Println("Ecoute sur https", strListenOn, "...")
	}

	//point d'entrée du ws
	router := httprouter.New()
	//création task
	router.POST("/task/queue", api.TaskCreateAsync)
	//getter état task
	router.GET("/task/queue/:tid", api.TaskGet)

	//ping health check
	router.GET("/task/ping", api.Ping)

	//gestion des erreurs qui provoquerait un crash (panic)
	router.PanicHandler = api.PanicHandler

	//mise en place de la gestion du ctrl-c
	server := &http.Server{ //server custom simplement pour avoir accés au shutdown
		Addr:    strListenOn,
		Handler: router,
	}
	//go routine en attente du ctrl-c
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		//ctrl-c emis
		<-interrupt
		slog.To("").Println("Ctrl-c recus, arret en cours...")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		//arret serveur web
		err := server.Shutdown(ctx)
		if err != nil {
			slog.To("").Println("server.Shutdown:", err)
		}
	}()

	//lancement du serveur web
	if bNoTLS {
		slog.To("").Fatal(server.ListenAndServe())
	} else {
		slog.To("").Fatal(server.ListenAndServeTLS(cert, key))
	}

}
