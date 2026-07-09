// Package infrastructure hosts the concrete SQL adapters that satisfy
// the domain's read model. All queries here are read-only — the
// example never writes.
package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jordanbrauer/hex/examples/swapi/domain"
)

// Repo is the SQL-backed read model over the swapi.db schema.
type Repo struct {
	db *sql.DB
}

// NewRepo constructs a Repo bound to db. db is not owned by Repo
// (opened + closed by hex/db/provider).
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// Counts returns row counts across the six resource tables in a
// single round-trip.
func (r *Repo) Counts(ctx context.Context) (domain.Counts, error) {
	const q = `
		SELECT
			(SELECT COUNT(*) FROM resources_film),
			(SELECT COUNT(*) FROM resources_people),
			(SELECT COUNT(*) FROM resources_planet),
			(SELECT COUNT(*) FROM resources_species),
			(SELECT COUNT(*) FROM resources_starship),
			(SELECT COUNT(*) FROM resources_vehicle)
	`

	var c domain.Counts

	err := r.db.QueryRowContext(ctx, q).Scan(
		&c.Films, &c.People, &c.Planets, &c.Species, &c.Starships, &c.Vehicles,
	)

	return c, err
}

// -- films ---------------------------------------------------------------

// ListFilms returns every film ordered by episode.
func (r *Repo) ListFilms(ctx context.Context) ([]domain.Film, error) {
	const q = `SELECT id, episode_id, title, director, producer, release_date, opening_crawl
		FROM resources_film ORDER BY episode_id`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list films: %w", err)
	}

	defer rows.Close()

	var out []domain.Film

	for rows.Next() {
		var (
			f      domain.Film
			relRaw string
			crawl  string
		)

		if err := rows.Scan(&f.ID, &f.EpisodeID, &f.Title, &f.Director, &f.Producer, &relRaw, &crawl); err != nil {
			return nil, err
		}

		f.ReleaseDate, _ = time.Parse("2006-01-02", relRaw)
		f.OpeningCrawl = crawl
		out = append(out, f)
	}

	return out, rows.Err()
}

// GetFilm fetches one film by id.
func (r *Repo) GetFilm(ctx context.Context, id int) (domain.Film, error) {
	const q = `SELECT id, episode_id, title, director, producer, release_date, opening_crawl
		FROM resources_film WHERE id = ?`

	var (
		f      domain.Film
		relRaw string
	)

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&f.ID, &f.EpisodeID, &f.Title, &f.Director, &f.Producer, &relRaw, &f.OpeningCrawl,
	)
	if err != nil {
		return f, err
	}

	f.ReleaseDate, _ = time.Parse("2006-01-02", relRaw)

	return f, nil
}

// FilmCharacters returns every person appearing in the film.
func (r *Repo) FilmCharacters(ctx context.Context, filmID int) ([]domain.Person, error) {
	const q = `SELECT p.id, p.name
		FROM resources_people p
		JOIN resources_film_characters fc ON fc.people_id = p.id
		WHERE fc.film_id = ?
		ORDER BY p.name`

	rows, err := r.db.QueryContext(ctx, q, filmID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Person

	for rows.Next() {
		var p domain.Person
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

// -- people --------------------------------------------------------------

// ListPeople returns every character with their homeworld's name.
func (r *Repo) ListPeople(ctx context.Context) ([]domain.Person, error) {
	const q = `SELECT p.id, p.name, p.height, p.mass, p.birth_year, p.gender,
			p.homeworld_id, IFNULL(w.name, '')
		FROM resources_people p
		LEFT JOIN resources_planet w ON w.id = p.homeworld_id
		ORDER BY p.name`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Person

	for rows.Next() {
		var p domain.Person

		if err := rows.Scan(
			&p.ID, &p.Name, &p.Height, &p.Mass, &p.BirthYear, &p.Gender,
			&p.HomeworldID, &p.Homeworld,
		); err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

// GetPerson fetches one character by id.
func (r *Repo) GetPerson(ctx context.Context, id int) (domain.Person, error) {
	const q = `SELECT p.id, p.name, p.height, p.mass, p.hair_color, p.skin_color,
			p.eye_color, p.birth_year, p.gender, p.homeworld_id, IFNULL(w.name, '')
		FROM resources_people p
		LEFT JOIN resources_planet w ON w.id = p.homeworld_id
		WHERE p.id = ?`

	var p domain.Person

	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.Name, &p.Height, &p.Mass, &p.HairColor, &p.SkinColor,
		&p.EyeColor, &p.BirthYear, &p.Gender, &p.HomeworldID, &p.Homeworld,
	)

	return p, err
}

// PersonFilms returns every film a character appears in.
func (r *Repo) PersonFilms(ctx context.Context, personID int) ([]domain.Film, error) {
	const q = `SELECT f.id, f.episode_id, f.title, f.director, f.producer, f.release_date
		FROM resources_film f
		JOIN resources_film_characters fc ON fc.film_id = f.id
		WHERE fc.people_id = ?
		ORDER BY f.episode_id`

	rows, err := r.db.QueryContext(ctx, q, personID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Film

	for rows.Next() {
		var (
			f      domain.Film
			relRaw string
		)

		if err := rows.Scan(&f.ID, &f.EpisodeID, &f.Title, &f.Director, &f.Producer, &relRaw); err != nil {
			return nil, err
		}

		f.ReleaseDate, _ = time.Parse("2006-01-02", relRaw)
		out = append(out, f)
	}

	return out, rows.Err()
}

// -- planets -------------------------------------------------------------

// ListPlanets returns every planet ordered by name.
func (r *Repo) ListPlanets(ctx context.Context) ([]domain.Planet, error) {
	const q = `SELECT id, name, climate, terrain, gravity, population,
			diameter, rotation_period, orbital_period, surface_water
		FROM resources_planet ORDER BY name`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Planet

	for rows.Next() {
		var p domain.Planet
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Climate, &p.Terrain, &p.Gravity, &p.Population,
			&p.Diameter, &p.RotationPeriod, &p.OrbitalPeriod, &p.SurfaceWater,
		); err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, rows.Err()
}

// -- species -------------------------------------------------------------

// ListSpecies returns every species with their homeworld name.
func (r *Repo) ListSpecies(ctx context.Context) ([]domain.Species, error) {
	const q = `SELECT s.id, s.name, s.classification, s.designation,
			s.average_height, s.average_lifespan, s.language,
			IFNULL(w.name, '')
		FROM resources_species s
		LEFT JOIN resources_planet w ON w.id = s.homeworld_id
		ORDER BY s.name`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Species

	for rows.Next() {
		var s domain.Species
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Classification, &s.Designation,
			&s.AverageHeight, &s.AverageLife, &s.Language, &s.Homeworld,
		); err != nil {
			return nil, err
		}

		out = append(out, s)
	}

	return out, rows.Err()
}

// -- starships & vehicles ------------------------------------------------

// ListStarships returns every starship with the joined transport row.
// SWAPI models starship as an inheritance from transport, so the join
// on transport_ptr_id is required to get name/model/crew.
func (r *Repo) ListStarships(ctx context.Context) ([]domain.Starship, error) {
	const q = `SELECT t.id, t.name, t.model, t.manufacturer, s.starship_class,
			s.hyperdrive_rating, s.MGLT, t.crew, t.passengers, t.cost_in_credits
		FROM resources_starship s
		JOIN resources_transport t ON t.id = s.transport_ptr_id
		ORDER BY t.name`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Starship

	for rows.Next() {
		var s domain.Starship
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Model, &s.Manufacturer, &s.StarshipClass,
			&s.HyperdriveRating, &s.MGLT, &s.Crew, &s.Passengers, &s.CostInCredits,
		); err != nil {
			return nil, err
		}

		out = append(out, s)
	}

	return out, rows.Err()
}

// ListVehicles returns every vehicle with the joined transport row.
func (r *Repo) ListVehicles(ctx context.Context) ([]domain.Vehicle, error) {
	const q = `SELECT t.id, t.name, t.model, t.manufacturer, v.vehicle_class,
			t.crew, t.passengers
		FROM resources_vehicle v
		JOIN resources_transport t ON t.id = v.transport_ptr_id
		ORDER BY t.name`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []domain.Vehicle

	for rows.Next() {
		var v domain.Vehicle
		if err := rows.Scan(
			&v.ID, &v.Name, &v.Model, &v.Manufacturer, &v.VehicleClass,
			&v.Crew, &v.Passengers,
		); err != nil {
			return nil, err
		}

		out = append(out, v)
	}

	return out, rows.Err()
}
