package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/ags131/serialportal/eventbus"
	"github.com/ags131/serialportal/portmanager"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var eb = eventbus.New()
var pm = portmanager.New()

func main() {
	files, err := ioutil.ReadDir("/dev")
	if err != nil {
		log.Fatal(err)
		return
	}
	for _, fi := range files {
		if strings.HasPrefix(fi.Name(), "ttyUSB") {
			log.Printf("Added dev %s", fi.Name())
			connect(fi.Name())
		}
	}
	go func() {
		watcher, err := fsnotify.NewWatcher()
		watcher.Add("/dev")
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Fatalf("Watcher failed: %v", err)
					return
				}
				dev := path.Base(event.Name)
				if strings.HasPrefix(dev, "ttyUSB") {
					log.Printf("%s: %s", event.Name, event.Op.String())
					if event.Op&fsnotify.Chmod == fsnotify.Chmod {
						connect(dev)
						log.Printf("Added dev %s", dev)
					}
					if event.Op&fsnotify.Remove == fsnotify.Remove {
						pm.Disconnect(dev)
						log.Printf("Removed dev %s", dev)
					}
				}
			}
		}
	}()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/serial/", func(w http.ResponseWriter, r *http.Request) {
		name := path.Base(r.URL.Path)
		log.Printf("%s: %s", r.URL, name)
		if name == "serial" {
			bytes, err := json.Marshal(map[string]interface{}{
				"deviceNames": pm.Ports(),
			})
			if err != nil {
				log.Print("Err with list", err)
			}
			w.Write(bytes)
		} else {
			if port := pm.Port(name); port != nil {
				handleSerialWS(w, r, port)
			} else {
				w.WriteHeader(404)
				w.Write([]byte(fmt.Sprintf("%s not found", name)))
				log.Printf("%s not found", name)
			}
		}
	})
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func connect(dev string) error {
	port, err := pm.Connect(dev, 115200)
	if err != nil {
		return err
	}
	go func() {
		for {
			buf := make([]byte, 1024)
			n, err := port.Socket.Read(buf)
			if n > 0 {
				eb.Publish(dev, buf[:n])
			}
			if err != nil {
				log.Printf("Serial read err %s: %v", dev, err)
				break
			}
		}
	}()
	return nil
}

func handleSerialWS(w http.ResponseWriter, r *http.Request, port *portmanager.Port) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		done1 := ctx.Done()
		done2 := port.Context().Done()
		ch := make(eventbus.DataChannel, 0)
		eb.Subscribe(port.Device, ch)
		defer func() {
			eb.UnSubscribe(port.Device, ch)
			log.Print("end")
			msg := "\u001b[41;38;1m\u001b[;H\u001b[J\u001b[12;37HDEVICE\u001b[13;34HDISCONNECTED"
			c.WriteMessage(websocket.BinaryMessage, []byte(msg))
			c.Close()
			close(ch)
		}()
		for {
			select {
			case ev := <-ch:
				c.WriteMessage(websocket.BinaryMessage, ev.Data.([]byte))
			case <-done1:
				return
			case <-done2:
				return
			}
		}
	}()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		port.Socket.Write(message)
	}
}
