package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gempir/go-twitch-irc"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type AudioCommand struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	File    string `json:"file"`
	TimeOut int64 `json:"timeout"`
	TimeoutExpires int64 `json:"-"`
}

type Config struct {
	AudioCommands []AudioCommand `json:"audioCommands"`
	TimeBetweenCommands int64    `json:"timeBetweenCommands"`
	LastCommandTime int64 `json:"lastTimeCommand"`
	Port          int            `json:"port"`
	Channel       string         `json:"channel"`
	Username      string         `json:"username,omitempty"`
	Password      string         `json:"password,omitempty"`
}

var config Config

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Failed to set websocket upgrade: %+v", err)
		return
	}

	defer conn.Close()

	client := twitch.NewAnonymousClient()
	client.OnPrivateMessage(func(pMsg twitch.PrivateMessage) {
		var data []byte
		var mimeType *mimetype.MIME
		var msgType int = websocket.TextMessage
		var err error
		now:=time.Now().Unix()
		if strings.HasPrefix(pMsg.Message, "!") {
			if (config.LastCommandTime + config.TimeBetweenCommands) >= now {
				// We aren't ready to do another audio command
				return
			}
			cmd := strings.TrimSpace(pMsg.Message)
			for i := range config.AudioCommands {
				if config.AudioCommands[i].Command == cmd &&
					config.AudioCommands[i].TimeoutExpires <= now {

					config.AudioCommands[i].TimeoutExpires = now + config.AudioCommands[i].TimeOut

					//data = []byte(config.AudioCommands[i].Name)
					mimeType,_ = mimetype.DetectFile(config.AudioCommands[i].File)
					data, err = ioutil.ReadFile(config.AudioCommands[i].File)

					if err != nil {
						log.Printf("Error: %s", err)
						break
					}


					break
				}
			}
			if err != nil {
				fmt.Printf("%s\n", err)
				return
			}
			if len(data) > 0 {
				b64 := base64.StdEncoding.EncodeToString(data)
				b64 = fmt.Sprintf("data:%s;base64,%s",mimeType.String(), b64)
				if err = conn.WriteMessage(msgType, []byte(b64)); err != nil {
					log.Printf("%s\n",err)
				}
				//log.Printf("executing audioCommand %s",data)
				config.LastCommandTime = now
			}
		}
	})

	client.Join(config.Channel)
	if err := client.Connect(); err != nil {
		panic(err)
	}
}

func main() {
	if b, err := ioutil.ReadFile("config.json"); err != nil {
		log.Fatal(err)
	} else {
		json.Unmarshal(b, &config)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.LoadHTMLFiles("index.html")
	r.Static("./audio", "audio")
	r.GET("/", func(context *gin.Context) {
		context.HTML(200, "index.html", config.AudioCommands)
	})
	r.GET("/ws", func(context *gin.Context) {
		wshandler(context.Writer, context.Request)
	})
	fmt.Printf("Listening on port %d",config.Port)
	r.Run(fmt.Sprintf(":%d", config.Port))
}
