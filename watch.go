package templates

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"text/template"

	"github.com/mazzegi/rwatch"
	"github.com/pkg/errors"
)

type Watcher struct {
	sync.RWMutex
	onErr   func(err error)
	templ   *template.Template
	root    string
	ext     string
	funcs   template.FuncMap
	watcher *rwatch.RecursiveWatcher
	doneC   chan struct{}
}

func NewWatcher(root string, ext string, onErr func(err error)) *Watcher {
	return &Watcher{
		onErr: onErr,
		root:  root,
		ext:   ext,
		doneC: make(chan struct{}),
	}
}

func (tw *Watcher) Activate(funcs template.FuncMap) error {
	tw.funcs = funcs
	err := tw.parseTemplates()
	if err != nil {
		return err
	}
	err = tw.watch()
	if err != nil {
		return err
	}
	return nil
}

func (tw *Watcher) Close() {
	tw.watcher.Close()
	<-tw.doneC
}

func (tw *Watcher) Execute(w io.Writer, name string, data interface{}, funcs template.FuncMap) error {
	tw.RLock()
	defer tw.RUnlock()
	tw.templ.Funcs(funcs)
	return tw.templ.ExecuteTemplate(w, name, data)
}

func (tw *Watcher) watch() error {
	w, err := rwatch.NewRecursiveWatcher(tw.root)
	if err != nil {
		return err
	}
	tw.watcher = w
	go func() {
		defer close(tw.doneC)
		for m := range w.Messages {
			switch m.(type) {
			case rwatch.Changed, rwatch.Created, rwatch.Deleted:
				err := tw.parseTemplates()
				if err != nil {
					tw.onErr(errors.Wrap(err, "parse-templates"))
				}
			}
		}
	}()
	return nil
}

func (tw *Watcher) parseTemplates() error {
	tw.Lock()
	defer tw.Unlock()
	tmpl := template.New("")
	tmpl.Funcs(tw.funcs)
	filenames := collectFiles(tw.root, tw.ext)
	_, err := tmpl.ParseFiles(filenames...)
	if err != nil {
		return errors.Wrap(err, "template: parse files")
	}
	tw.templ = tmpl
	return nil
}

func collectFiles(root string, ext string) []string {
	filenames := []string{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ext {
			filenames = append(filenames, path)
		}
		return nil
	})
	return filenames
}
