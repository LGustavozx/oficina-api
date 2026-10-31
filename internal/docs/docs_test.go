package docs_test

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"oficina-api/internal/app"
	"oficina-api/internal/docs"
	"oficina-api/internal/platform/config"
	"oficina-api/internal/shared/clock"
)

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromData(docs.Spec())
	if err != nil {
		t.Fatalf("o openapi.yaml não carrega: %v", err)
	}
	return doc
}

// operations devolve "MÉTODO /caminho" para cada operação documentada.
func operations(doc *openapi3.T) map[string]*openapi3.Operation {
	out := map[string]*openapi3.Operation{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			out[method+" "+path] = op
		}
	}
	return out
}

func TestSpecIsAValidOpenAPIDocument(t *testing.T) {
	doc := loadSpec(t)

	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("o openapi.yaml não é um documento OpenAPI válido: %v", err)
	}
}

func TestEveryOperationIsDescribed(t *testing.T) {
	for key, op := range operations(loadSpec(t)) {
		t.Run(key, func(t *testing.T) {
			if op.Summary == "" {
				t.Error("falta o summary")
			}
			if len(op.Tags) == 0 {
				t.Error("falta a tag")
			}

			hasSuccess := false
			for status := range op.Responses.Map() {
				if strings.HasPrefix(status, "2") {
					hasSuccess = true
				}
			}
			if !hasSuccess {
				t.Error("falta a resposta de sucesso (2xx)")
			}
		})
	}
}

// Rotas registradas no código: padrões como "GET /admin/customers/{id}".
var routePattern = regexp.MustCompile(`"(GET|POST|PUT|DELETE|PATCH) (/[^"\s]*)"`)

func routesInSource(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}

	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		slashed := filepath.ToSlash(path)
		if !strings.Contains(slashed, "/http/") && !strings.HasSuffix(slashed, "platform/httpserver/server.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range routePattern.FindAllStringSubmatch(string(content), -1) {
			found[m[1]+" "+m[2]] = slashed
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// Se uma rota existe no código e não na documentação (ou o contrário), este
// teste falha: assim o openapi.yaml, escrito à mão, não fica desatualizado.
func TestSpecMatchesTheRoutesInTheCode(t *testing.T) {
	documented := operations(loadSpec(t))
	inCode := routesInSource(t)

	if len(inCode) == 0 {
		t.Fatal("nenhuma rota encontrada no código: o teste está lendo o lugar errado")
	}

	var missingInDocs, missingInCode []string
	for route, file := range inCode {
		if _, ok := documented[route]; !ok {
			missingInDocs = append(missingInDocs, route+" (em "+file+")")
		}
	}
	for route := range documented {
		if _, ok := inCode[route]; !ok {
			missingInCode = append(missingInCode, route)
		}
	}

	sort.Strings(missingInDocs)
	sort.Strings(missingInCode)
	if len(missingInDocs) > 0 {
		t.Errorf("rotas do código que faltam no openapi.yaml:\n  %s", strings.Join(missingInDocs, "\n  "))
	}
	if len(missingInCode) > 0 {
		t.Errorf("rotas do openapi.yaml que não existem no código:\n  %s", strings.Join(missingInCode, "\n  "))
	}
}

// buildAPI monta a API completa. O banco é só um endereço que nunca responde:
// as chamadas deste teste não passam da autenticação nem da validação.
func buildAPI(t *testing.T) http.Handler {
	t.Helper()
	db, err := sql.Open("pgx", "postgres://x:x@127.0.0.1:1/x?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := config.Config{JWTSecret: "0123456789abcdef0123456789abcdef", JWTTTL: time.Hour}
	handler, err := app.NewHandler(context.Background(), db, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), clock.System{})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

// Cada operação documentada precisa existir na API de verdade: sem
// credenciais, a resposta pode ser 400, 401 ou 503, mas nunca 404 ou 405 do roteador.
func TestEveryDocumentedOperationIsServed(t *testing.T) {
	handler := buildAPI(t)
	id := uuid.NewString()

	for key := range operations(loadSpec(t)) {
		t.Run(key, func(t *testing.T) {
			method, path, _ := strings.Cut(key, " ")
			path = strings.NewReplacer("{id}", id, "{number}", "1").Replace(path)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

			if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
				t.Errorf("%s respondeu %d: a operação documentada não existe na API", key, rec.Code)
			}
		})
	}
}

func TestSpecAndDocsPageAreServed(t *testing.T) {
	handler := buildAPI(t)

	spec := httptest.NewRecorder()
	handler.ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if spec.Code != http.StatusOK || !strings.HasPrefix(spec.Header().Get("Content-Type"), "application/yaml") {
		t.Errorf("/openapi.yaml: %d %s", spec.Code, spec.Header().Get("Content-Type"))
	}
	if spec.Body.String() != string(docs.Spec()) {
		t.Error("o /openapi.yaml servido difere do arquivo embutido")
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/docs", nil))
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("/docs: %d %s", page.Code, page.Header().Get("Content-Type"))
	}
	for _, want := range []string{`url: "/openapi.yaml"`, "swagger-ui-bundle.js", "swagger-ui.css"} {
		if !strings.Contains(body, want) {
			t.Errorf("a página /docs deveria conter %q", want)
		}
	}
}

// Os arquivos externos da página têm de ter versão fixa e assinatura SRI.
func TestDocsPageLocksExternalAssets(t *testing.T) {
	handler := buildAPI(t)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/docs", nil))
	body := page.Body.String()

	external := regexp.MustCompile(`(?:src|href)="(https://[^"]+)"[^>]*`).FindAllStringSubmatch(body, -1)
	if len(external) == 0 {
		t.Fatal("esperava recursos externos na página")
	}
	for _, m := range external {
		if !regexp.MustCompile(`@\d+\.\d+\.\d+/`).MatchString(m[1]) {
			t.Errorf("%s não tem versão fixa", m[1])
		}
		if !strings.Contains(m[0], `integrity="sha384-`) || !strings.Contains(m[0], `crossorigin="anonymous"`) {
			t.Errorf("%s não tem integridade (SRI)", m[1])
		}
	}
}
