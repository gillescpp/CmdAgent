package slog

import (
	"io"
	"log"
	"os"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

//group de log
var logGrp map[string]*log.Logger

// InitLogs initialise les logs sous forme de groupe identifier par une chaine
//group="" est utilisé pour les logs de l'application elle même
func InitLogs(group string, filename string, maxSize int, maxBackup int, compression bool) {
	if logGrp == nil {
		logGrp = make(map[string]*log.Logger)
	}

	//instance writer à appliquer
	if filename == "" {
		//stdout
		logGrp[group] = log.New(os.Stdout, group, log.LstdFlags)
	} else {
		//fichier
		w := &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize,
			MaxBackups: maxBackup,
			Compress:   compression,
			LocalTime:  true,
		}
		if group == "" {
			//log principaux : stdout quoi qu'il arrive
			w2 := io.MultiWriter(os.Stdout, w)
			logGrp[group] = log.New(w2, "", log.LstdFlags) //pas de prefix sur le log fichiers
		} else {
			logGrp[group] = log.New(w, "", log.LstdFlags) //pas de prefix sur le log fichiers
		}
	}
}

//To getter logger taches ("" pour l'agent lui même)
func To(group string) *log.Logger {
	if _, exists := logGrp[group]; exists {
		return logGrp[group]
	}
	return logGrp["_default_"]
}

//GrpExists permete de controler l'existance d'un groupe de config
func GrpExists(group string) bool {
	if group == "" {
		return true //vide ok : config par defaut appliqué
	} else if _, exists := logGrp[group]; exists {
		return true
	}
	return false
}
