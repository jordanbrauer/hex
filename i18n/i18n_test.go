package i18n_test

import (
	"embed"
	"testing"

	"github.com/jordanbrauer/hex/i18n"
)

//go:embed testdata
var fixtures embed.FS

func newTranslator(t *testing.T) *i18n.Translator {
	t.Helper()

	tr, err := i18n.NewTranslator(i18n.Options{
		FS:        fixtures,
		Root:      "testdata",
		Languages: []string{"en", "es", "fr"},
		Domains:   []string{"messages", "errors"},
		Fallback:  "en",
	})
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}

	return tr
}

func TestNewTranslator_requiresLanguages(t *testing.T) {
	_, err := i18n.NewTranslator(i18n.Options{
		FS:   fixtures,
		Root: "testdata",
	})
	if err == nil {
		t.Errorf("empty Languages returned nil error")
	}
}

func TestNewTranslator_fallbackMustBeLoaded(t *testing.T) {
	_, err := i18n.NewTranslator(i18n.Options{
		FS:        fixtures,
		Root:      "testdata",
		Languages: []string{"en"},
		Fallback:  "de", // not loaded
	})
	if err == nil {
		t.Errorf("unloaded Fallback returned nil error")
	}
}

func TestT_basicTranslation(t *testing.T) {
	tr := newTranslator(t)

	if err := tr.Use("es"); err != nil {
		t.Fatal(err)
	}

	if got := tr.T("Hello, world"); got != "Hola, mundo" {
		t.Errorf("T = %q, want Hola, mundo", got)
	}

	if err := tr.Use("fr"); err != nil {
		t.Fatal(err)
	}

	if got := tr.T("Hello, world"); got != "Bonjour le monde" {
		t.Errorf("fr T = %q", got)
	}
}

func TestTN_pluralForm(t *testing.T) {
	tr := newTranslator(t)
	_ = tr.Use("es")

	tests := []struct {
		n    int
		want string
	}{
		{1, "1 manzana"},
		{2, "2 manzanas"},
		{5, "5 manzanas"},
	}

	for _, tt := range tests {
		got := tr.TN("%d apple", "%d apples", tt.n, tt.n)
		if got != tt.want {
			t.Errorf("TN(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestTC_contextDisambiguates(t *testing.T) {
	tr := newTranslator(t)
	_ = tr.Use("es")

	if got := tr.TC("File", "menu"); got != "Archivo" {
		t.Errorf("TC(File, menu) = %q, want Archivo", got)
	}

	if got := tr.TC("File", "verb"); got != "Archivar este informe" {
		t.Errorf("TC(File, verb) = %q, want Archivar este informe", got)
	}
}

func TestTD_secondaryDomain(t *testing.T) {
	tr := newTranslator(t)
	_ = tr.Use("es")

	if got := tr.TD("errors", "not found"); got != "no encontrado" {
		t.Errorf("TD(errors, not found) = %q", got)
	}

	if got := tr.TD("errors", "unauthorized"); got != "no autorizado" {
		t.Errorf("TD(errors, unauthorized) = %q", got)
	}
}

func TestT_missingTranslationReturnsMsgID(t *testing.T) {
	tr := newTranslator(t)
	_ = tr.Use("es")

	if got := tr.T("This key does not exist"); got != "This key does not exist" {
		t.Errorf("missing key = %q, want the msgid itself", got)
	}
}

func TestUse_unknownLanguageFails(t *testing.T) {
	tr := newTranslator(t)

	if err := tr.Use("de"); err == nil {
		t.Errorf("Use(unloaded) returned nil error")
	}
}

func TestLanguages_returnsSorted(t *testing.T) {
	tr := newTranslator(t)
	got := tr.Languages()

	want := []string{"en", "es", "fr"}
	if len(got) != len(want) {
		t.Fatalf("Languages = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Languages[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCurrentAndFallback(t *testing.T) {
	tr := newTranslator(t)

	if got := tr.Current(); got != "en" {
		t.Errorf("Current initial = %q, want en (default = fallback)", got)
	}

	if got := tr.Fallback(); got != "en" {
		t.Errorf("Fallback = %q, want en", got)
	}

	_ = tr.Use("es")

	if got := tr.Current(); got != "es" {
		t.Errorf("Current after Use(es) = %q", got)
	}
}

func TestLocale_returnsPerLanguageBundle(t *testing.T) {
	tr := newTranslator(t)

	if got := tr.Locale("es"); got == nil {
		t.Errorf("Locale(es) returned nil")
	}

	if got := tr.Locale("nope"); got != nil {
		t.Errorf("Locale(unloaded) = %v, want nil", got)
	}
}

func TestNewEmbedded_shortcut(t *testing.T) {
	tr, err := i18n.NewEmbedded(fixtures, "testdata", []string{"en", "es"}, "messages")
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}

	_ = tr.Use("es")

	if got := tr.T("Hello, world"); got != "Hola, mundo" {
		t.Errorf("NewEmbedded T = %q", got)
	}
}

func TestIsTranslated(t *testing.T) {
	tr := newTranslator(t)
	_ = tr.Use("es")

	if !tr.IsTranslated("Hello, world") {
		t.Errorf("IsTranslated(Hello, world) = false")
	}

	if tr.IsTranslated("Never translated key") {
		t.Errorf("IsTranslated(nonexistent) = true")
	}
}

// -- Package-level API ----------------------------------------------------

func TestPackageLevel_zeroValueReturnsMsgID(t *testing.T) {
	i18n.SetDefault(nil)

	if got := i18n.T("hello"); got != "hello" {
		t.Errorf("T w/o default = %q, want raw msgid", got)
	}

	if got := i18n.TN("one", "many", 5); got != "many" {
		t.Errorf("TN w/o default (n=5) = %q, want plural", got)
	}

	if got := i18n.TN("one", "many", 1); got != "one" {
		t.Errorf("TN w/o default (n=1) = %q, want singular", got)
	}
}

func TestPackageLevel_delegatesToDefault(t *testing.T) {
	tr := newTranslator(t)

	i18n.SetDefault(tr)

	t.Cleanup(func() { i18n.SetDefault(nil) })

	if err := i18n.Use("es"); err != nil {
		t.Fatalf("Use: %v", err)
	}

	if got := i18n.T("Hello, world"); got != "Hola, mundo" {
		t.Errorf("T = %q", got)
	}

	if got := i18n.TN("%d apple", "%d apples", 3, 3); got != "3 manzanas" {
		t.Errorf("TN = %q", got)
	}

	if got := i18n.TC("File", "menu"); got != "Archivo" {
		t.Errorf("TC = %q", got)
	}

	if got := i18n.TD("errors", "not found"); got != "no encontrado" {
		t.Errorf("TD = %q", got)
	}
}

func TestPackageLevel_useWithoutDefaultErrors(t *testing.T) {
	i18n.SetDefault(nil)

	if err := i18n.Use("es"); err == nil {
		t.Errorf("Use w/o default returned nil error")
	}
}
