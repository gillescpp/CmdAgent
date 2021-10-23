package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

//Config instance globale config
var Config AppConfig

//AppConfigLogStorage contient les configs pour les logs
type AppConfigLogStorage struct {
	MaxSizeMB  int    //taille d'un log
	MaxBackups int    //nombre max de log (rotation)
	Compress   bool   //Compression
	LogFolder  string //dossier pour le stockage des logs
}

//normalise les valeurs d'un AppConfigLogStorage
func (c *AppConfigLogStorage) normalise() {
	if c.LogFolder == "" {
		c.MaxSizeMB = 0
		c.MaxBackups = 0
		c.Compress = false
	}
	if c.MaxSizeMB < 0 {
		c.MaxSizeMB = 0
	}
	if c.MaxBackups < 0 {
		c.MaxBackups = 0
	}
}

//AppConfig contient les configs propres à l'agent
type AppConfig struct {
	ListenPort int    //port d'ecoute
	APIKey     string //api key a fournir
	NoTLS      bool

	LogStdConfig  *AppConfigLogStorage            //config standard pour les logs
	LogTaskConfig map[string]*AppConfigLogStorage //config task
}

//InitConfig charge la config
func InitConfig() error {
	//api key
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Errorf("rand %w", err)
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	//chemin appli
	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Exec path : %w", err)
	}
	exePath := filepath.Dir(ex)
	exeLogPath := filepath.Join(exePath, "log")

	//config par defaut
	defCfg := AppConfig{
		ListenPort: 8800,
		NoTLS:      false,
		APIKey:     uuid,
		LogStdConfig: &AppConfigLogStorage{
			MaxSizeMB:  20,
			MaxBackups: 1,
			Compress:   false,
			LogFolder:  exeLogPath,
		},
		LogTaskConfig: map[string]*AppConfigLogStorage{
			"_default_": {
				MaxSizeMB:  20,
				MaxBackups: 3,
				Compress:   false,
				LogFolder:  exeLogPath,
			},
		},
	}

	//chargement config ou création
	cfgPath := filepath.Join(exePath, "CmdAgent.json")
	bToUpdate := false

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		//n'existe pas :
		Config = defCfg
		bToUpdate = true
	} else if err != nil {
		//autre erreur
		return err
	} else {
		//deserialise
		buffer, err := ioutil.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("ReadFile %s : %w", cfgPath, err)
		}
		err = json.Unmarshal(buffer, &Config)
		if err != nil {
			return fmt.Errorf("Unmarshal %s : %w", cfgPath, err)
		}
	}

	//controle input
	if Config.ListenPort <= 0 || Config.ListenPort > 65535 {
		Config.ListenPort = defCfg.ListenPort
		bToUpdate = true
	}

	Config.LogStdConfig.normalise()
	if Config.LogStdConfig.LogFolder != "" { //destination vide = stdout
		err = os.MkdirAll(Config.LogStdConfig.LogFolder, os.ModePerm)
		if err != nil {
			return fmt.Errorf("Make dir %s : %w", Config.LogStdConfig.LogFolder, err)
		}
	}
	if Config.LogTaskConfig == nil {
		Config.LogTaskConfig = make(map[string]*AppConfigLogStorage)
	}
	//log app obligatoire
	if _, exists := Config.LogTaskConfig["_default_"]; !exists {
		Config.LogTaskConfig["_default_"] = defCfg.LogTaskConfig["_default_"]
		bToUpdate = true
	}
	for _, v := range Config.LogTaskConfig {
		v.normalise()
		if v.LogFolder != "" {
			err = os.MkdirAll(v.LogFolder, os.ModePerm)
			if err != nil {
				return fmt.Errorf("Make dir %s : %w", v.LogFolder, err)
			}
		}

	}

	//save cfg
	if bToUpdate {
		buffer, err := json.MarshalIndent(&Config, "", " ")
		if err != nil {
			return fmt.Errorf("MarshalIndent %s : %w", cfgPath, err)
		}
		err = ioutil.WriteFile(cfgPath, buffer, 0644)
		if err != nil {
			return fmt.Errorf("WriteFile %s : %w", cfgPath, err)
		}
	}

	return nil
}
