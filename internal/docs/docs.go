// Package docs serve a documentação OpenAPI da API e a interface Swagger UI.
package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var spec []byte

// O Swagger UI vem de uma CDN, fixado em uma versão e conferido por SRI: o
// navegador recusa o arquivo se ele não tiver exatamente o conteúdo esperado.
const page = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>oficina-api — documentação</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.1/swagger-ui.css"
        integrity="sha384-Ov4/wv3j2bmct8cDc5X4ngJZohVPzEmc6uDPH8WeljUxO5vtoykvMEfbu9Vh6RaW" crossorigin="anonymous">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.1/swagger-ui-bundle.js"
          integrity="sha384-ZPehFMQommnnuaZ4rpxgkgTT2DKFVp4hZC/7pLit+9Lek9T1YGSo23eHFbvNkXkw" crossorigin="anonymous"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui", deepLinking: true });
  </script>
</body>
</html>
`

// Spec devolve o conteúdo do openapi.yaml embutido.
func Spec() []byte { return spec }

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(spec)
	})

	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
}
