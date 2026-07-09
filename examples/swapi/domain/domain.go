// Package domain holds the swapi resources as plain Go types.
// SWAPI's original schema stores physical attributes as VARCHARs
// (values include "n/a", "unknown", "1.7m", …), so the numeric-looking
// fields are kept as strings — the point of this example is to render
// them, not to normalise the data.
package domain

import "time"

// Film is one entry in the Star Wars saga.
type Film struct {
	ID           int
	EpisodeID    int
	Title        string
	Director     string
	Producer     string
	ReleaseDate  time.Time
	OpeningCrawl string // Markdown-rendered by the show view
}

// Person is a character.
type Person struct {
	ID          int
	Name        string
	Height      string
	Mass        string
	HairColor   string
	SkinColor   string
	EyeColor    string
	BirthYear   string
	Gender      string
	HomeworldID int
	Homeworld   string // planet name (joined from resources_planet)
}

// Planet is a world.
type Planet struct {
	ID             int
	Name           string
	Climate        string
	Terrain        string
	Gravity        string
	Population     string
	Diameter       string
	RotationPeriod string
	OrbitalPeriod  string
	SurfaceWater   string
}

// Species is a sentient species.
type Species struct {
	ID             int
	Name           string
	Classification string
	Designation    string
	AverageHeight  string
	AverageLife    string
	Language       string
	Homeworld      string // joined planet name, may be empty
}

// Starship is a hyperspace-capable vessel.
type Starship struct {
	ID               int
	Name             string
	Model            string
	Manufacturer     string
	StarshipClass    string
	HyperdriveRating string
	MGLT             string
	Crew             string
	Passengers       string
	CostInCredits    string
}

// Vehicle is a non-hyperspace transport.
type Vehicle struct {
	ID           int
	Name         string
	Model        string
	Manufacturer string
	VehicleClass string
	Crew         string
	Passengers   string
}

// Counts is the tuple shown on the landing page.
type Counts struct {
	Films     int
	People    int
	Planets   int
	Species   int
	Starships int
	Vehicles  int
}
