package main

import (
	"encoding/csv"
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jaschaephraim/lrserver"
	"gopkg.in/fsnotify.v1"
)

type Result struct {
	Query_e       string
	ErrorOccurred bool
	ErrorMessage  string
	Content       [][]string
	Time          string
}

func getCSVContent(filename string, exclude []string) (ret [][]string, err error) {
	fp, err := os.Open(filename)
	if err != nil {
		return ret, err
	}
	defer fp.Close()

	reader := csv.NewReader(fp)
	for {
		r, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return ret, err
		}
		var skip bool
		for _, s := range exclude {
			if r[0] == s {
				skip = true
			}
		}
		if !skip {
			ret = append(ret, r)
		}
	}
	return ret, nil
}

const html = `<!doctype html>
<html>
<head>
<title>Result</title>
<link rel="stylesheet" type="text/css" href="style.css?t={{.Time}}">
</head>
<body>
<div>
<h1>Result</h1>
{{if .ErrorOccurred}}
<p style="color: red;">ERROR:<br>{{.ErrorMessage}}</p>
{{else}}
{{if ne .Query_e ""}}<p>e={{.Query_e}}</p>{{end}}
<table border="1" cellpadding="2" style="border-collapse: collapse;">
{{range $ii, $vv := .Content}}
	<tr class="{{index . 0}} row-{{$ii}}">
	{{range $i, $v := .}}
		<td class="col-{{$i}}">{{$v}}</td>
	{{end}}
	</tr>
{{end}}
</table>
{{end}}
</div>
<script src="http://localhost:35729/livereload.js"></script>
</body>
</html>`

const defaultCSS = `/* example css */

/*
tr.row-0 { text-align: center; }

td.col-0, td.col-1 { text-align: center; width: 30px; }

tr.A { background-color: lightpink; }
tr.B { background-color: peachpuff; }
tr.E { background-color: khaki; }
*/
`

var (
	tmpl          = template.Must(template.New("").Parse(html))
	validquerypat = regexp.MustCompile(`^[a-zA-Z]+$`)
	useCustomCSS  = false
	port          = flag.String("p", "3000", `Port Number`)
	customCSSPath = flag.String("s", "", `Custom CSS File Path (ex. ./style.css)`)
	targetCSVFile = flag.String("t", "", `Target CSV File Path (required)`)
)

func main() {
	flag.Parse()
	if *customCSSPath != "" {
		useCustomCSS = true
	}
	if *targetCSVFile == "" {
		log.Fatalln("-t option required")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err)
	}
	defer watcher.Close()

	err = watcher.Add(filepath.Dir(*targetCSVFile))
	if err != nil {
		log.Fatalln(err)
	}

	lr := lrserver.New(lrserver.DefaultName, lrserver.DefaultPort)
	go lr.ListenAndServe()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println(event.Op, event.Name)
				if runtime.GOOS == "linux" && event.Op&fsnotify.Write == fsnotify.Write {
					lr.Reload(event.Name)
				}
				if runtime.GOOS == "windows" && event.Op&fsnotify.Write == fsnotify.Write && event.Name == *targetCSVFile {
					lr.Reload(event.Name)
				}
				//if event.Op&fsnotify.Write == fsnotify.Write && event.Name == *targetCSVFile {
				//	lr.Reload(event.Name)
				//}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println(err)
			}
		}
	}()

	http.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		t := time.Now().Format(time.RFC3339)
		exclude := make([]string, 0)
		if e := r.URL.Query().Get("e"); e != "" {
			if !validquerypat.MatchString(e) {
				tmpl.Execute(w, Result{
					ErrorOccurred: true,
					ErrorMessage:  "Invalid Query: " + e,
					Time:          t,
				})
				return
			}
			for _, c := range e {
				exclude = append(exclude, strings.ToUpper(string(c)))
			}
		}
		log.Println("exclude:", exclude)
		content, err := getCSVContent(*targetCSVFile, exclude)
		if err != nil {
			log.Println(err)

			tmpl.Execute(w, Result{
				ErrorOccurred: true,
				ErrorMessage:  err.Error(),
				Time:          t,
			})
			return
		}
		tmpl.Execute(w, Result{
			Query_e: r.URL.Query().Get("e"),
			Content: content,
			Time:    t,
		})
	})

	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css")
		if useCustomCSS {
			http.ServeFile(w, r, *customCSSPath)
		} else {
			w.Write([]byte(defaultCSS))
		}
	})
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
