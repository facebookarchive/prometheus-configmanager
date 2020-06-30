/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package client

import (
	"fmt"
	"prometheus-configmanager/fsclient"
	"prometheus-configmanager/prometheus/alert"
	"reflect"
	"sort"
	"strings"
	"unsafe"

	"text/template"

	"github.com/thoas/go-funk"
)

const TemplateFilePostfix = ".tmpl"

// TemplateClient interface provides methods for modifying template files
// and individual templates within them
type TemplateClient interface {
	GetTemplateFile(filename string) (string, error)
	CreateTemplateFile(filename, fileText string) error
	EditTemplateFile(filename, fileText string) error
	DeleteTemplateFile(filename string) error

	GetTemplates(filename string) (map[string]string, error)

	GetTemplate(filename, tmplName string) (string, error)
	AddTemplate(filename, tmplName, tmplText string) error
	EditTemplate(filename, tmplName, tmplText string) error
	DeleteTemplate(filename, tmplName string) error

	Root() string
}

func NewTemplateClient(fsClient fsclient.FSClient, fileLocks *alert.FileLocker) TemplateClient {
	return &templateClient{
		fsClient:  fsClient,
		fileLocks: fileLocks,
	}
}

type templateClient struct {
	templateDir string
	fsClient    fsclient.FSClient
	fileLocks   *alert.FileLocker
}

func (t *templateClient) GetTemplateFile(filename string) (string, error) {
	t.fileLocks.RLock(filename)
	defer t.fileLocks.RUnlock(filename)

	file, err := t.fsClient.ReadFile(addFilePostfix(filename))
	if err != nil {
		return "", err
	}

	return string(file), nil
}

func (t *templateClient) CreateTemplateFile(filename, fileText string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	return t.fsClient.WriteFile(addFilePostfix(filename), []byte(fileText), 0660)
}

func (t *templateClient) EditTemplateFile(filename, fileText string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	return t.fsClient.WriteFile(addFilePostfix(filename), []byte(fileText), 0660)
}

func (t *templateClient) DeleteTemplateFile(filename string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	return t.fsClient.DeleteFile(addFilePostfix(filename))
}

func (t *templateClient) GetTemplates(filename string) (map[string]string, error) {
	t.fileLocks.RLock(filename)
	defer t.fileLocks.RUnlock(filename)

	tmpl, err := t.readTmplFile(filename)
	if err != nil {
		return nil, err
	}

	tmplMap := getTemplatesByName(tmpl)

	tmplTextMap := make(map[string]string, len(tmplMap))
	for key, t := range tmplMap {
		// Don't include template for entire file
		if t.Name() == t.ParseName {
			continue
		}
		tmplTextMap[key] = writeTemplateText(t)
	}
	return tmplTextMap, nil
}

func (t *templateClient) GetTemplate(filename, tmplName string) (string, error) {
	t.fileLocks.RLock(filename)
	defer t.fileLocks.RUnlock(filename)

	tmplFile, err := t.readTmplFile(filename)
	if err != nil {
		return "", err
	}
	tmplMap := getTemplatesByName(tmplFile)

	tmpl := tmplMap[tmplName]
	if tmpl == nil {
		return "", fmt.Errorf("template %s not found", tmplName)
	}

	return writeTemplateText(tmpl), nil
}

func (t *templateClient) AddTemplate(filename, tmplName, tmplText string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	tmplFile, err := t.readTmplFile(filename)
	if err != nil {
		return err
	}
	tmplMap := getTemplatesByName(tmplFile)

	if tmplMap[tmplName] != nil {
		return fmt.Errorf("template %s already exists", tmplName)
	}

	newTmpl := &template.Template{}
	newTmplBody, err := newTmpl.Parse(tmplText)
	tmplMap[tmplName] = newTmplBody

	return t.writeTmplFile(filename, writeTmplMapText(tmplMap))
}

func (t *templateClient) EditTemplate(filename, tmplName, tmplText string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	tmplFile, err := t.readTmplFile(filename)
	if err != nil {
		return err
	}
	tmplMap := getTemplatesByName(tmplFile)

	if tmplMap[tmplName] == nil {
		return fmt.Errorf("template %s does not exist", tmplName)
	}

	parseTmpl := &template.Template{}
	newTmpl, err := parseTmpl.Parse(tmplText)
	if err != nil {
		return fmt.Errorf("error adding template: %v", err)
	}
	tmplMap[tmplName] = newTmpl

	return t.writeTmplFile(filename, writeTmplMapText(tmplMap))
}

func (t *templateClient) DeleteTemplate(filename, tmplName string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	tmplFile, err := t.readTmplFile(filename)
	if err != nil {
		return err
	}
	tmplMap := getTemplatesByName(tmplFile)

	if tmplMap[tmplName] == nil {
		return fmt.Errorf("template %s does not exist", tmplName)
	}

	delete(tmplMap, tmplName)

	return t.writeTmplFile(filename, writeTmplMapText(tmplMap))
}

func (t *templateClient) Root() string {
	return t.fsClient.Root()
}

func (t *templateClient) writeTmplFile(filename, text string) error {
	err := t.fsClient.WriteFile(addFilePostfix(filename), []byte(text), 0660)
	if err != nil {
		return fmt.Errorf("error writing template file: %v", err)
	}
	return nil
}

func (t *templateClient) readTmplFile(filename string) (*template.Template, error) {
	tmplFile, err := template.ParseFiles(t.Root() + addFilePostfix(filename))
	if err != nil {
		return nil, fmt.Errorf("error parsing template files: %v", err)
	}
	return tmplFile, nil
}

func addFilePostfix(filename string) string {
	return filename + TemplateFilePostfix
}

func writeTemplateText(tmpl *template.Template) string {
	return tmpl.Root.String()
}

func writeTmplMapText(tmplMap map[string]*template.Template) string {
	str := strings.Builder{}
	// Sort names for consistency
	names := funk.Keys(tmplMap).([]string)
	//names := make([]string, 0)
	//for key := range tmplMap {
	//	names = append(names, key)
	//}
	sort.Strings(names)

	for _, name := range names {
		tmpl := tmplMap[name]
		if name == tmpl.Tree.ParseName {
			continue
		}
		str.WriteString(defineTemplate(name, tmpl.Root.String()))
		str.WriteRune('\n')
	}
	return str.String()
}

func defineTemplate(tmplName, tmplText string) string {
	return fmt.Sprintf(`{{ define "%s" }}%s{{ end }}`, tmplName, tmplText)
}

func getTemplatesByName(tmpl *template.Template) map[string]*template.Template {
	field := reflect.ValueOf(tmpl).Elem().FieldByName("tmpl")
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(map[string]*template.Template)
}
