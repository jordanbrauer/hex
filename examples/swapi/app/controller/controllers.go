package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/jordanbrauer/hex/examples/swapi/infrastructure"
)

// Resources bundles every controller in the app. One struct so wiring
// is a single container.Make instead of six.
type Resources struct {
	repo *infrastructure.Repo
}

// NewResources builds the resource-controller bundle.
func NewResources(repo *infrastructure.Repo) *Resources { return &Resources{repo: repo} }

// Home renders the landing page with per-resource counts.
func (r *Resources) Home(c echo.Context) error {
	counts, err := r.repo.Counts(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/home", map[string]any{
		"Title":  "swapi",
		"Counts": counts,
	})
}

// -- films ---------------------------------------------------------------

// Films lists every film ordered by episode.
func (r *Resources) Films(c echo.Context) error {
	films, err := r.repo.ListFilms(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/films/index", map[string]any{
		"Title": "Films",
		"Films": films,
	})
}

// Film renders one film with its opening crawl (Markdown) and cast.
func (r *Resources) Film(c echo.Context) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}

	film, err := r.repo.GetFilm(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "film not found")
	}

	cast, err := r.repo.FilmCharacters(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/films/show", map[string]any{
		"Title": film.Title,
		"Film":  film,
		"Cast":  cast,
		"Crawl": crawlParagraphs(film.OpeningCrawl),
	})
}

// crawlParagraphs splits the SWAPI opening_crawl ("\r\n\r\n"
// paragraph-separated in the source data) into rendered paragraphs.
func crawlParagraphs(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n\n")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	return out
}

// -- people --------------------------------------------------------------

// People lists every character.
func (r *Resources) People(c echo.Context) error {
	people, err := r.repo.ListPeople(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/people/index", map[string]any{
		"Title":  "People",
		"People": people,
	})
}

// Person renders one character plus the films they appear in.
func (r *Resources) Person(c echo.Context) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}

	person, err := r.repo.GetPerson(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "person not found")
	}

	films, err := r.repo.PersonFilms(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/people/show", map[string]any{
		"Title":  person.Name,
		"Person": person,
		"Films":  films,
	})
}

// -- planets / species / starships / vehicles ---------------------------

// Planets lists every planet.
func (r *Resources) Planets(c echo.Context) error {
	planets, err := r.repo.ListPlanets(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/planets/index", map[string]any{
		"Title":   "Planets",
		"Planets": planets,
	})
}

// Species lists every species.
func (r *Resources) Species(c echo.Context) error {
	species, err := r.repo.ListSpecies(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/species/index", map[string]any{
		"Title":   "Species",
		"Species": species,
	})
}

// Starships lists every starship.
func (r *Resources) Starships(c echo.Context) error {
	starships, err := r.repo.ListStarships(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/starships/index", map[string]any{
		"Title":     "Starships",
		"Starships": starships,
	})
}

// Vehicles lists every vehicle.
func (r *Resources) Vehicles(c echo.Context) error {
	vehicles, err := r.repo.ListVehicles(c.Request().Context())
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "pages/vehicles/index", map[string]any{
		"Title":    "Vehicles",
		"Vehicles": vehicles,
	})
}

func paramID(c echo.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	return id, nil
}
