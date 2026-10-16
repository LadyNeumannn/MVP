package handler

import (
	"log"
	"net/http"
	"os"
)

const swaggerUIPage = `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8" />
  <title>Авито.Кухня — API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        urls: [
          { url: "/docs/client-api.yaml", name: "Client API (для web-клиента)" },
          { url: "/docs/partner-api.yaml", name: "Partner API (для заведений)" }
        ],
        "urls.primaryName": "Client API (для web-клиента)",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
    };
  </script>
</body>
</html>`

func ServeDocsUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}

func ServeOpenAPISpec(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Запрос спецификации: %s", filename)
		data, err := os.ReadFile("api/" + filename)
		if err != nil {
			log.Printf("Ошибка чтения файла api/%s: %v", filename, err)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write(data)
	}
}
