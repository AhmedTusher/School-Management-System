// gallery.go - photos added by admins to the public Gallery page.
// Items are stored in data/gallery.json and uploaded image files are saved
// inside images/gallery/. The public gallery.html page reads /api/gallery.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// One photo card on the Gallery page.
type GalleryItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
}

func (a *App) galleryFile() string {
	return filepath.Join(filepath.Dir(a.file), "gallery.json")
}

// Reads gallery.json at start-up (empty list if none added yet).
func (a *App) loadGallery() error {
	data, err := os.ReadFile(a.galleryFile())
	if err == nil {
		return json.Unmarshal(data, &a.gallery)
	}
	if os.IsNotExist(err) {
		a.gallery = nil
		return nil
	}
	return err
}

// Saves gallery.json safely (temporary file, then rename).
func (a *App) saveGallery() error {
	dir := filepath.Dir(a.galleryFile())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.gallery, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "gallery-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, a.galleryFile())
}

// Saves an uploaded image (JPEG, PNG or WebP up to 5 MB) into images/gallery/.
func (a *App) saveUploadedGalleryPhoto(r *http.Request, field string) (string, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil || len(raw) == 0 || len(raw) > 5<<20 {
		return "", os.ErrInvalid
	}
	ext := ""
	switch http.DetectContentType(raw) {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		return "", os.ErrInvalid
	}
	rel := filepath.ToSlash(filepath.Join("images", "gallery", "g-"+token()[:12]+ext))
	dest := filepath.Join(a.frontendDir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, raw, 0644); err != nil {
		return "", err
	}
	return rel, nil
}

// Saves an uploaded PDF file (up to 5 MB) into downloads/results/.
func (a *App) saveUploadedResultPDF(r *http.Request, field string) (string, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil || len(raw) < 5 || len(raw) > 5<<20 || !bytes.HasPrefix(raw, []byte("%PDF-")) {
		return "", os.ErrInvalid
	}
	rel := filepath.ToSlash(filepath.Join("downloads", "results", "res-"+token()[:12]+".pdf"))
	dest := filepath.Join(a.frontendDir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, raw, 0644); err != nil {
		return "", err
	}
	return rel, nil
}

// Saves an uploaded admission document or photo (PDF, JPEG, PNG or WebP up to 5 MB) into downloads/admissions/.
func (a *App) saveUploadedAdmissionFile(r *http.Request, field string) (string, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil || len(raw) < 5 || len(raw) > 5<<20 {
		return "", os.ErrInvalid
	}
	ext := ""
	if bytes.HasPrefix(raw, []byte("%PDF-")) {
		ext = ".pdf"
	} else {
		switch http.DetectContentType(raw) {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		default:
			return "", os.ErrInvalid
		}
	}
	rel := filepath.ToSlash(filepath.Join("downloads", "admissions", "adm-"+token()[:12]+ext))
	dest := filepath.Join(a.frontendDir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, raw, 0644); err != nil {
		return "", err
	}
	return rel, nil
}

