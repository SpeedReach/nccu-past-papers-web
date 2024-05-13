package app

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	b64 "encoding/base64"

	"past-papers-web/internal/helper"
)

func (a *App) uploadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	file, header, err := r.FormFile("file")
	if err != nil {
		fmt.Println("Error getting file", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	dst := make([]byte, b64.StdEncoding.EncodedLen(len(buf.Bytes())))
	b64.StdEncoding.Strict().Encode(dst, buf.Bytes())

	newBranchName := "upload-from-" + name
	newBranchSha, err := a.helper.CreateBranch(newBranchName)
	if err != nil {
		fmt.Println("Error creating branch", err)
	}

	uploadData := helper.UploadData{
		Message: "Upload from " + name,
		Content: string(dst),
		Branch:  newBranchName,
		Sha:     newBranchSha,
	}
	err = a.helper.Upload(&uploadData, r.URL.Path[len("/content/"):]+"/"+header.Filename)
	if err != nil {
		fmt.Println("Error uploading file", err)
	}

	err = a.helper.CreatePR(newBranchName)
	if err != nil {
		fmt.Println("Error creating PR", err)
	}

	// w.WriteHeader(http.StatusOK)
	data := map[string]interface{}{
		"Redirect": r.URL.Path[len("/content/"):],
	}
	tmpl := template.Must(template.ParseFiles("templates/success.html"))
	tmpl.Execute(w, data)
	// http.Redirect(w, r, r.URL.Path[len("/content/"):], http.StatusSeeOther)
	return
}

func (a *App) handlePDFFile(w http.ResponseWriter, r *http.Request) {
	urlpath := r.URL.Path[len("/content/"):]
	pdfData, err := a.helper.GetFile(urlpath)
	if err != nil {
		fmt.Fprintf(w, "%s", err)
	}

	b := bytes.NewBuffer(pdfData)

	w.Header().Set("Content-type", "application/pdf")
	if _, err := b.WriteTo(w); err != nil {
		fmt.Fprintf(w, "%s", err)
	}

	w.Write([]byte("PDF Generated"))
	return
}

func (a *App) HandleContent(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.uploadFile(w, r)
		return
	}

	// Get the path from the URL
	urlpath := r.URL.Path[len("/content/"):]

	if strings.HasSuffix(urlpath, ".pdf") {
		a.handlePDFFile(w, r)
		return
	}

	type Item struct {
		Link   string
		Name   string
		IsTree bool
	}

	res := a.helper.GetStructure()
	items := make([]Item, 0)

	for _, v := range res["tree"].([]interface{}) {
		if treeItem, ok := v.(map[string]interface{}); ok {
			if path, ok := treeItem["path"].(string); ok {
				if strings.HasPrefix(path, urlpath) && len(strings.Split(path, "/")) == len(strings.Split(urlpath, "/"))+1 {
					lnk := strings.Split(urlpath, "/")[len(strings.Split(urlpath, "/"))-1] + "/" + strings.Split(path, "/")[len(strings.Split(urlpath, "/"))]
					items = append(items, Item{
						Link:   lnk,
						Name:   lnk,
						IsTree: treeItem["type"].(string) == "tree",
					})
				}
			}
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/content.html"))
	content := map[string]interface{}{
		"Title": "Content " + urlpath,
		"Items": items,
	}

	err := tmpl.Execute(w, content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return
}