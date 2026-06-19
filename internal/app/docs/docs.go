package docs

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, swaggerJSON string) {
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerJSON))
	})

	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(getScalarHTML()))
	})
}

func getScalarHTML() string {
	return `<!doctype html>
<html>
  <head>
    <title>Notion Clone API Docs</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
      body { margin: 0; padding: 0; background-color: #0f172a; }
      #scalar-ui { height: 100vh; }
    </style>
  </head>
  <body>
    <!-- Элемент, куда смонтируется интерфейс документации -->
    <div id="scalar-ui"></div>

    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
    <script>
      // Правильный синтаксис CDN-версии Scalar
      Scalar.createApiReference('#scalar-ui', {
        url: '/openapi.json',
        theme: 'purple',
        layout: 'modern'
      });
    </script>
  </body>
</html>`
}
