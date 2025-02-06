package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/amadrigalIstmo/pokedexcli/pokecache"
)

type Pokemon struct {
	Name           string `json:"name"`
	BaseExperience int    `json:"base_experience"`
	Height         int    `json:"height"`
	Weight         int    `json:"weight"`
	Stats          []struct {
		BaseStat int `json:"base_stat"`
		Stat     struct {
			Name string `json:"name"`
		} `json:"stat"`
	} `json:"stats"`
	Types []struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
}

// Config para mantener el estado de paginación
type config struct {
	nextURL     *string
	previousURL *string
	cache       *pokecache.Cache
	pokedex     map[string]Pokemon // Nuevo campo para almacenar Pokémon
}

// Añade estas estructuras para parsear la respuesta de la API
type locationAreaResponse struct {
	PokemonEncounters []struct {
		Pokemon struct {
			Name string `json:"name"`
		} `json:"pokemon"`
	} `json:"pokemon_encounters"`
}

// Estructuras para parsear la respuesta de la API
type locationAreasResponse struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Results  []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"results"`
}

type cliCommand struct {
	name        string
	description string
	callback    func(*config, []string) error // ← Acepta argumentos
}

var commands map[string]cliCommand

func commandMap(cfg *config, args []string) error {
	url := "https://pokeapi.co/api/v2/location-area/"
	if cfg.nextURL != nil {
		url = *cfg.nextURL
	}

	// 1. Verificar caché
	if data, ok := cfg.cache.Get(url); ok {
		fmt.Println("(usando caché)")
		return parseLocationData(data, cfg)
	}

	// 2. Hacer solicitud HTTP
	resp, err := http.Get(url)
	if err != nil { // ← Aquí USAMOS 'err'
		return fmt.Errorf("error al hacer la solicitud: %v", err)
	}
	defer resp.Body.Close()

	// 3. Leer el cuerpo
	body, err := io.ReadAll(resp.Body)
	if err != nil { // ← Aquí USAMOS 'err'
		return fmt.Errorf("error leyendo el cuerpo: %v", err)
	}

	// 4. Guardar en caché
	cfg.cache.Add(url, body)

	return parseLocationData(body, cfg)
}

// Función helper para parsear
func parseLocationData(data []byte, cfg *config) error {
	var locations locationAreasResponse
	if err := json.Unmarshal(data, &locations); err != nil {
		return err
	}

	cfg.nextURL = locations.Next
	cfg.previousURL = locations.Previous

	for _, area := range locations.Results {
		fmt.Println(area.Name)
	}
	return nil
}

func commandMapB(cfg *config, args []string) error {
	if cfg.previousURL == nil {
		fmt.Println("You're on the first page")
		return nil
	}

	resp, err := http.Get(*cfg.previousURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var locations locationAreasResponse
	if err := json.NewDecoder(resp.Body).Decode(&locations); err != nil {
		return err
	}

	cfg.nextURL = locations.Next
	cfg.previousURL = locations.Previous

	for _, area := range locations.Results {
		fmt.Println(area.Name)
	}
	return nil
}

// Define las funciones ANTES de inicializar el mapa
func commandHelp(cfg *config, args []string) error {
	fmt.Println("\nAvailable commands:")
	for _, cmd := range commands {
		fmt.Printf("  %-10s %s\n", cmd.name+":", cmd.description)
	}
	fmt.Println()
	return nil
}

func commandExit(cfg *config, args []string) error {
	fmt.Println("\nClosing the Pokedex... Goodbye!")
	os.Exit(0)
	return nil
}

// Inicializa el mapa en una función init()
func init() {
	commands = map[string]cliCommand{
		// ... otros comandos
		"explore": {
			name:        "explore",
			description: "Explore a location area for Pokémon",
			callback:    commandExplore,
		},
		"help": {
			name:        "help",
			description: "Displays a help message",
			callback:    commandHelp, // ← Ahora apunta a la función actualizada
		},
		"exit": {
			name:        "exit",
			description: "Exit the Pokedex",
			callback:    commandExit, // ← Ahora apunta a la función actualizada
		},
		"map": {
			name:        "map",
			description: "Displays next 20 location areas",
			callback:    commandMap,
		},
		"mapb": {
			name:        "mapb",
			description: "Displays previous 20 location areas",
			callback:    commandMapB,
		},
		"catch": {
			name:        "catch",
			description: "Attempt to catch a Pokémon",
			callback:    commandCatch,
		},
		"inspect": {
			name:        "inspect",
			description: "Inspect a caught Pokémon",
			callback:    commandInspect,
		},
		"pokedex": {
			name:        "pokedex",
			description: "List all caught Pokémon",
			callback:    commandPokedex,
		},
	}
}

// El resto del código (main y cleanInput) igual que antes...
// ... [tu código existente para main() y cleanInput()]
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	cfg := &config{
		cache:   pokecache.NewCache(5 * time.Minute),
		pokedex: make(map[string]Pokemon),
	}

	fmt.Println("Welcome to the Pokedex! Type 'help' for available commands")

	for {
		fmt.Print("Pokedex > ")
		scanner.Scan()

		input := scanner.Text()
		cleaned := cleanInput(input)

		if len(cleaned) == 0 {
			continue
		}

		commandName := cleaned[0]
		command, exists := commands[commandName]
		if exists {
			err := command.callback(cfg, cleaned[1:])
			if err != nil {
				fmt.Println("Error:", err)
			}
		} else {
			fmt.Println("Unknown command. Type 'help' for available commands")
		}
	}
}

func cleanInput(text string) []string {
	trimmed := strings.TrimSpace(text)
	lowered := strings.ToLower(trimmed)
	return strings.Fields(lowered)
}

func commandExplore(cfg *config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing location area name")
	}

	areaName := args[0]
	url := fmt.Sprintf("https://pokeapi.co/api/v2/location-area/%s", areaName)

	// Verificar caché
	var data []byte
	if cached, ok := cfg.cache.Get(url); ok {
		fmt.Println("(usando caché)")
		data = cached
	} else {
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("error fetching data: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API error: %s", resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response: %v", err)
		}

		data = body
		cfg.cache.Add(url, body)
	}

	var area locationAreaResponse
	if err := json.Unmarshal(data, &area); err != nil {
		return fmt.Errorf("error parsing data: %v", err)
	}

	fmt.Printf("Pokémon en %s:\n", areaName)
	for _, encounter := range area.PokemonEncounters {
		fmt.Printf(" - %s\n", encounter.Pokemon.Name)
	}

	return nil
}

func commandCatch(cfg *config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing Pokémon name")
	}

	pokemonName := strings.ToLower(args[0])
	fmt.Printf("Throwing a Pokeball at %s...\n", pokemonName)

	// Obtener datos del Pokémon
	url := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%s", pokemonName)

	var data []byte
	if cached, ok := cfg.cache.Get(url); ok {
		data = cached
	} else {
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("error fetching Pokémon: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("Pokémon '%s' not found", pokemonName)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response: %v", err)
		}

		data = body
		cfg.cache.Add(url, body)
	}

	var pokemon Pokemon
	if err := json.Unmarshal(data, &pokemon); err != nil {
		return fmt.Errorf("error parsing Pokémon data: %v", err)
	}

	// Calcular probabilidad de captura (ej: 50% para 100 base exp)
	catchChance := 100.0 / (1.0 + float64(pokemon.BaseExperience)/100.0)
	rand.Seed(time.Now().UnixNano())

	if rand.Float64()*100 < catchChance {
		cfg.pokedex[pokemon.Name] = pokemon
		fmt.Printf("%s was caught!\n", pokemon.Name)
		fmt.Println("You may now inspect it with the inspect command.")
	} else {
		fmt.Printf("%s escaped!\n", pokemon.Name)
	}

	return nil
}

func commandInspect(cfg *config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing Pokémon name")
	}

	pokemonName := strings.ToLower(args[0])
	pokemon, exists := cfg.pokedex[pokemonName]
	if !exists {
		fmt.Println("you have not caught that pokemon")
		return nil
	}

	fmt.Printf("Name: %s\n", pokemon.Name)
	fmt.Printf("Height: %d\n", pokemon.Height)
	fmt.Printf("Weight: %d\n", pokemon.Weight)

	fmt.Println("Stats:")
	for _, stat := range pokemon.Stats {
		fmt.Printf("  -%s: %d\n", stat.Stat.Name, stat.BaseStat)
	}

	fmt.Println("Types:")
	for _, t := range pokemon.Types {
		fmt.Printf("  - %s\n", t.Type.Name)
	}

	return nil
}

func commandPokedex(cfg *config, args []string) error {
	fmt.Println("Your Pokedex:")

	if len(cfg.pokedex) == 0 {
		fmt.Println("  You haven't caught any Pokémon yet!")
		return nil
	}

	for name := range cfg.pokedex {
		fmt.Printf(" - %s\n", name)
	}
	return nil
}
