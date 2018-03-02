// Copyright 2018 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package chatserver

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context" // Use this until Go 1.9's type alias is available
	"google.golang.org/appengine"
	"google.golang.org/appengine/memcache"
)

const (
	messagesKey           = "messages"
	maxContentSizeInBytes = 256
)

type Message struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

const (
	messagesHTMLTmpl = `<!DOCTYPE html>
<title>Chat Server - golang.tokyo #13</title>
<style>
body {
  font-family: Sans-Serif;
}
.name {
  font-weight: bold;
}
</style>
<script>
window.onload = () => {
  setTimeout(() => {
    location.reload();
  }, 5000);
};
</script>
{{range .Messages -}}
<div><span class="name">{{.Name}}</span>: {{.Body}}</div>
{{else}}
No Message!
{{- end}}
`

	devForm = `<!DOCTYPE html>
<script>
window.addEventListener('load', _ => {
  document.getElementById('submit-button').addEventListener('click', _ => {
    let name = document.getElementById('name').value;
    let body = document.getElementById('body').value;
    fetch('/messages', {
      method: 'POST',
      body:   JSON.stringify({'name': name, 'body': body}),
    }).then(response => {
      console.log('status:', response.status);
      return response.text();
    });
  });
});
</script>
Name: <input id="name" type="text">
Body: <input id="body" type="text">
<button id="submit-button">Submit</button>
`
)

var (
	messagesHTML = template.Must(template.New("messages").Parse(messagesHTMLTmpl))
)

func getMessages(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/dev":
		if appengine.IsDevAppServer() {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, devForm)
			return
		}

	case "/", "/messages", "/messages.html":
		messages := []Message{}
		if _, err := memcache.JSON.Get(ctx, messagesKey, &messages); err != nil {
			if err != memcache.ErrCacheMiss {
				msg := fmt.Sprintf("Memcache error: %v", err)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			}
		}

		// Reverse
		messagesToShow := make([]Message, len(messages))
		for i, m := range messages {
			messagesToShow[len(messages)-i-1] = m
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		messagesHTML.Execute(w, map[string]interface{}{
			"Messages": messagesToShow,
		})
		return
	}

	http.NotFound(w, r)
}

func postMessages(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/messages" {
		http.NotFound(w, r)
		return
	}

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := fmt.Sprintf("Could not read the request body: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if len(reqBody) > maxContentSizeInBytes {
		msg := "Request body is too big"
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	message := Message{}
	if err := json.Unmarshal(reqBody, &message); err != nil {
		msg := fmt.Sprintf("Unmarshal JSON error: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var messages []Message
	item, err := memcache.JSON.Get(ctx, messagesKey, &messages)
	if err != nil {
		if err != memcache.ErrCacheMiss {
			msg := fmt.Sprintf("Memcache error: %v", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		item := &memcache.Item{
			Key:    messagesKey,
			Object: []Message{message},
		}
		if err := memcache.JSON.Set(ctx, item); err != nil {
			msg := fmt.Sprintf("Memcache error: %v", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	messages = append(messages, message)
	const maxMessageNum = 50
	if len(messages) > maxMessageNum {
		messages = messages[len(messages)-maxMessageNum:]
	}
	item.Object = messages

	if err := memcache.JSON.CompareAndSwap(ctx, item); err != nil {
		msg := fmt.Sprintf("Could not store the request body: %v", err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func handleSnippets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := appengine.NewContext(r)
	switch r.Method {
	case http.MethodHead, http.MethodGet:
		getMessages(ctx, w, r)
	case http.MethodPost:
		postMessages(ctx, w, r)
	default:
		s := http.StatusMethodNotAllowed
		http.Error(w, http.StatusText(s), s)
	}
}

func init() {
	http.HandleFunc("/", handleSnippets)
}
