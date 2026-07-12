package http

import stdhttp "net/http"

func registerDocs(mux *stdhttp.ServeMux, swaggerJSON string) {
	mux.HandleFunc("/openapi.json", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(swaggerJSON))
	})

	mux.HandleFunc("/docs", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(stdhttp.StatusOK)
		_, _ = w.Write([]byte(scalarHTML))
	})
}

const scalarHTML = `<!doctype html>
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
    <div id="scalar-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
    <script>
      Scalar.createApiReference('#scalar-ui', {
        url: '/openapi.json',
        theme: 'purple',
        layout: 'modern'
      });
    </script>
  </body>
</html>`
