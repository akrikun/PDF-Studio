package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"

	"pdf-studio/internal/config"
	"pdf-studio/internal/models"
)

type PDFService struct {
	Cfg        *config.Config
	StorageSvc *StorageService
}

func NewPDFService(cfg *config.Config, storageSvc *StorageService) *PDFService {
	return &PDFService{Cfg: cfg, StorageSvc: storageSvc}
}

const pageTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
@page {
  size: A4;
  margin: 20mm;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: 'Helvetica Neue', Arial, sans-serif;
  font-size: 12pt;
  line-height: 1.5;
  color: #1a1a1a;
}
.page {
  width: 170mm;
  min-height: 257mm;
  padding: 0;
  page-break-after: always;
}
.page:last-child { page-break-after: auto; }
h1 { font-size: 24pt; margin-bottom: 12pt; }
h2 { font-size: 18pt; margin-bottom: 10pt; }
h3 { font-size: 14pt; margin-bottom: 8pt; }
p { margin-bottom: 8pt; }
img { max-width: 100%%; height: auto; }
table { border-collapse: collapse; width: 100%%; margin-bottom: 12pt; }
th, td { border: 1px solid #ccc; padding: 6pt 8pt; text-align: left; }
th { background: #f5f5f5; font-weight: bold; }
ul, ol { margin-left: 20pt; margin-bottom: 8pt; }
blockquote { border-left: 3pt solid #ccc; padding-left: 10pt; margin: 8pt 0; color: #555; }
</style>
</head>
<body>
{{range .Pages}}
<div class="page">{{.HTML}}</div>
{{end}}
</body>
</html>`

// pdfPageData holds pre-processed page HTML as template.HTML (safe, already processed)
type pdfPageData struct {
	HTML template.HTML
}

type pdfData struct {
	Pages []pdfPageData
}

// imgSrcRe matches src="/api/files/UUID/download" in img tags
var imgSrcRe = regexp.MustCompile(`(<img\b[^>]*\bsrc=")(/api/files/([0-9a-f\-]{36})/download)("[^>]*>)`)

// inlineImages replaces all /api/files/UUID/download image sources with base64 data URIs
func (s *PDFService) inlineImages(html string) string {
	return imgSrcRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := imgSrcRe.FindStringSubmatch(match)
		if len(parts) < 5 {
			return match
		}
		prefix := parts[1]  // <img ... src="
		fileID := parts[3]  // UUID
		suffix := parts[4]  // " ...>

		dataURI, err := s.fileToDataURI(fileID)
		if err != nil {
			log.Printf("[pdf] failed to inline image %s: %v", fileID, err)
			return match // leave original src, will be broken in PDF but not crash
		}

		return prefix + dataURI + suffix
	})
}

func (s *PDFService) fileToDataURI(fileID string) (string, error) {
	f, err := s.StorageSvc.GetFileByIDInternal(fileID)
	if err != nil {
		return "", fmt.Errorf("file lookup: %w", err)
	}

	data, err := s.StorageSvc.GetFileContent(f)
	if err != nil {
		return "", fmt.Errorf("file read: %w", err)
	}

	mime := f.Mime
	if mime == "" || mime == "application/octet-stream" {
		// Guess from common extensions
		name := strings.ToLower(f.OriginalName)
		switch {
		case strings.HasSuffix(name, ".png"):
			mime = "image/png"
		case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
			mime = "image/jpeg"
		case strings.HasSuffix(name, ".gif"):
			mime = "image/gif"
		case strings.HasSuffix(name, ".webp"):
			mime = "image/webp"
		case strings.HasSuffix(name, ".svg"):
			mime = "image/svg+xml"
		default:
			mime = "image/png"
		}
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return "data:" + mime + ";base64," + b64, nil
}

func (s *PDFService) GeneratePDF(pages []models.Page) ([]byte, error) {
	tmpl, err := template.New("pdf").Parse(pageTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// Process pages: inline all images as base64
	pdfPages := make([]pdfPageData, len(pages))
	for i, p := range pages {
		processed := s.inlineImages(p.ContentHTML)
		pdfPages[i] = pdfPageData{HTML: template.HTML(processed)}
	}

	var htmlBuf bytes.Buffer
	if err := tmpl.Execute(&htmlBuf, pdfData{Pages: pdfPages}); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	pdfBytes, err := s.callGotenberg(htmlBuf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gotenberg: %w", err)
	}

	return pdfBytes, nil
}

func (s *PDFService) callGotenberg(htmlContent []byte) ([]byte, error) {
	url := s.Cfg.GotenbergURL + "/forms/chromium/convert/html"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(htmlContent); err != nil {
		return nil, err
	}

	writer.WriteField("paperWidth", "8.27")
	writer.WriteField("paperHeight", "11.69")
	writer.WriteField("marginTop", "0.39")
	writer.WriteField("marginBottom", "0.39")
	writer.WriteField("marginLeft", "0.39")
	writer.WriteField("marginRight", "0.39")
	writer.WriteField("preferCssPageSize", "true")

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gotenberg returned %d: %s", resp.StatusCode, string(errBody))
	}

	return io.ReadAll(resp.Body)
}
