package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Configuration file struct
type ConfigurationFile struct {
	InputFile          string                           `yaml:"input_file"`
	OutputFile         string                           `yaml:"output_file"`
	OutputDirectory    string                           `yaml:"output_directory"`
	Ticks              uint64                           `yaml:"ticks"`
	DisableWraparound  bool                             `yaml:"disable_wraparound"`
	WorldDimensions    ConfigurationFileWorldDimensions `yaml:"world_dimensions"`
	NewLifeSpawn       []int                            `yaml:"new_life_spawn"`
	ExistingLifeRemain []int                            `yaml:"existing_life_remain"`
}

type ConfigurationFileWorldDimensions struct {
	X ConfigurationFileWorldDimensionsMeasurements `yaml:"x"`
	Y ConfigurationFileWorldDimensionsMeasurements `yaml:"y"`
}

type ConfigurationFileWorldDimensionsMeasurements struct {
	Minimum int64 `yaml:"minimum"`
	Maximum int64 `yaml:"maximum"`
}

// Prepare all CLI flags as well as application derived variables from CLI flags & configuration file
var (
	flagConfigurationFile  string
	flagShowHelp           bool
	flagInputFile          string
	flagOutputFile         string
	flagOutputDirectory    string
	flagTicks              uint64
	flagDisableWraparound  bool
	flagWorldDimensions    string
	flagNewLifeSpawn       string
	flagExistingLifeRemain string

	appInputFile          string
	appOutputFile         string
	appOutputDirectory    string
	appTicks              uint64
	appWraparound         bool
	appWorldMinX          int64
	appWorldMaxX          int64
	appWorldMinY          int64
	appWorldMaxY          int64
	appNewLifeSpawn       []int
	appExistingLifeRemain []int

	ticksDigitsLength uint64
)

// Init default application vars and flags system
func init() {
	appTicks = 10
	appWraparound = true
	appWorldMinX = math.MinInt64
	appWorldMaxX = math.MaxInt64
	appWorldMinY = math.MinInt64
	appWorldMaxY = math.MaxInt64
	appNewLifeSpawn = []int{3}
	appExistingLifeRemain = []int{2, 3}

	flag.StringVar(&flagConfigurationFile, "configuration", "", "Path to configuration file to use")
	flag.BoolVar(&flagShowHelp, "help", false, "Show help")
	flag.StringVar(&flagInputFile, "input", "", "Input file to use rather than stdin")
	flag.StringVar(&flagOutputFile, "output", "", "Output file to use rather than stdout")
	flag.StringVar(&flagOutputDirectory, "outdir", "", "Output directory to log all ticks")
	flag.Uint64Var(&flagTicks, "ticks", 0, "Number of ticks to run")
	flag.BoolVar(&flagDisableWraparound, "nowrap", false, "Disables wrapping the world around at the edges")
	flag.StringVar(&flagWorldDimensions, "world", "", "The dimensions of the world; in format min-x:max-x;min-y:max-y")
	flag.StringVar(&flagNewLifeSpawn, "newlife", "", "How many neighbors are required for new life to spawn; comma-delimited integer format")
	flag.StringVar(&flagExistingLifeRemain, "exlife", "", "How many neighbors are required for existing life to remain; comma-delimited integer format")
}

func main() {
	// Setup
	parseFlags()
	processConfigurationFile()
	processConfigurationCli()
	bootstrap()

	organisms := seedLife()

	// Run the simulation
	var tick uint64

	for tick = 0; tick < appTicks; tick++ {
		outputOrganismsTick(organisms, tick)

		organismsNext := make(map[int64]map[int64]int)

		for coordX, coordYs := range organisms {
			coordXLeft, coordXRight, coordXLeftExists, coordXRightExists := getNeighborsX(coordX)

			for coordY, alive := range coordYs {
				coordYBottom, coordYTop, coordYBottomExists, coordYTopExists := getNeighborsY(coordY)

				// How many neighbors?
				neighborsAlive := 0

				if coordXLeftExists {
					neighborsAlive += hasLife(organisms, coordXLeft, coordY)

					if coordYBottomExists {
						neighborsAlive += hasLife(organisms, coordXLeft, coordYBottom)
					}

					if coordYTopExists {
						neighborsAlive += hasLife(organisms, coordXLeft, coordYTop)
					}
				}

				if coordXRightExists {
					neighborsAlive += hasLife(organisms, coordXRight, coordY)

					if coordYBottomExists {
						neighborsAlive += hasLife(organisms, coordXRight, coordYBottom)
					}

					if coordYTopExists {
						neighborsAlive += hasLife(organisms, coordXRight, coordYTop)
					}
				}

				if coordYBottomExists {
					neighborsAlive += hasLife(organisms, coordX, coordYBottom)
				}
				if coordYTopExists {
					neighborsAlive += hasLife(organisms, coordX, coordYTop)
				}

				// Coordinates will be alive *by default* if:
				// 1. Already alive and 2-3 live neighbors
				// 2. Not alive and 3 live neighbors
				// This can be configured via CLI parameters
				add := false

				var neighborsCheck *[]int

				if alive == 1 {
					neighborsCheck = &appExistingLifeRemain
				} else {
					neighborsCheck = &appNewLifeSpawn
				}

				for _, neighbors := range *neighborsCheck {
					if neighborsAlive == neighbors {
						add = true

						break
					}
				}

				if add {
					addOrganism(organismsNext, coordX, coordY)
				}
			}
		}

		organisms = organismsNext
	}

	// And we're done; let's wrap up
	outputOrganismsTick(organisms, appTicks)

	var file io.Writer
	var err error

	if appOutputFile != "" {
		if file, err = os.OpenFile(appOutputFile, os.O_RDWR|os.O_CREATE, 0755); err != nil {
			log.Fatal("Error opening output file:", appOutputFile)
		}
	} else {
		file = os.Stdout
	}

	outputOrganisms(organisms, file)
	os.Exit(0)
}

// Parses CLI flags
func parseFlags() {
	flag.Parse()

	if flagShowHelp {
		flag.Usage()

		os.Exit(0)
	}
}

// Processes the configuration file and applies any settings
func processConfigurationFile() {
	if flagConfigurationFile != "" {
		// Parse & validate
		configurationFileBody, err := os.ReadFile(flagConfigurationFile)

		if err != nil {
			log.Fatal("CLI flags error: Error reading configuration file:", err)
		}

		var cf ConfigurationFile

		if err = yaml.Unmarshal(configurationFileBody, &cf); err != nil {
			log.Fatal("CLI flags error: Error decoding configuration YAML:", err)
		}

		// Process values
		if cf.InputFile != "" {
			appInputFile = cf.InputFile
		}

		if cf.OutputFile != "" {
			appOutputFile = cf.OutputFile
		}

		if cf.OutputDirectory != "" {
			appOutputDirectory = cf.OutputDirectory
		}

		if cf.Ticks != 0 {
			appTicks = cf.Ticks
		}

		if cf.DisableWraparound {
			appWraparound = false
		}

		if cf.WorldDimensions.X.Minimum != 0 || cf.WorldDimensions.X.Maximum != 0 || cf.WorldDimensions.Y.Minimum != 0 || cf.WorldDimensions.Y.Maximum != 0 {
			appWorldMinX = cf.WorldDimensions.X.Minimum
			appWorldMaxX = cf.WorldDimensions.X.Maximum
			appWorldMinY = cf.WorldDimensions.Y.Minimum
			appWorldMaxY = cf.WorldDimensions.X.Maximum
		}

		if len(cf.NewLifeSpawn) > 0 {
			appNewLifeSpawn = cf.NewLifeSpawn
		}

		if len(cf.ExistingLifeRemain) > 0 {
			appExistingLifeRemain = cf.ExistingLifeRemain
		}
	}
}

// Processes the configuration passed by the CLI
func processConfigurationCli() {
	if flagInputFile != "" {
		appInputFile = flagInputFile
	}

	if flagOutputFile != "" {
		appOutputFile = flagOutputFile
	}

	if flagOutputDirectory != "" {
		appOutputDirectory = flagOutputDirectory
	}

	if flagTicks != 0 {
		appTicks = flagTicks
	}

	if !flagDisableWraparound {
		appWraparound = false
	}

	if flagWorldDimensions != "" {
		parts := strings.Split(flagWorldDimensions, ";")
		numberParts := len(parts)

		if numberParts != 2 {
			log.Fatal("CLI flags error: Incorrect number of dimensions in world:", numberParts)
		}

		partsX := strings.Split(parts[0], ":")
		numberPartsX := len(partsX)

		if numberPartsX != 2 {
			log.Fatal("CLI flags error: Incorrect number of directions in X dimension:", numberPartsX)
		}

		partsY := strings.Split(parts[1], ":")
		numberPartsY := len(partsY)

		if numberPartsY != 2 {
			log.Fatal("CLI flags error: Incorrect number of directions in X dimension:", numberPartsY)
		}

		var err error

		appWorldMinX, err = strconv.ParseInt(strings.TrimSpace(partsX[0]), 10, 64)

		if err != nil {
			log.Fatalf("CLI flags error: Unable to parse world X dimension minimum %s: %s", partsX[0], err)
		}

		appWorldMaxX, err = strconv.ParseInt(strings.TrimSpace(partsX[1]), 10, 64)

		if err != nil {
			log.Fatalf("CLI flags error: Unable to parse world X dimension maximum %s: %s", partsX[1], err)
		}

		appWorldMinY, err = strconv.ParseInt(strings.TrimSpace(partsY[0]), 10, 64)

		if err != nil {
			log.Fatalf("CLI flags error: Unable to parse world Y dimension minimum %s: %s", partsY[0], err)
		}

		appWorldMaxY, err = strconv.ParseInt(strings.TrimSpace(partsY[1]), 10, 64)

		if err != nil {
			log.Fatalf("CLI flags error: Unable to parse world Y dimension maximum %s: %s", partsY[1], err)
		}
	}

	if flagNewLifeSpawn != "" {
		appNewLifeSpawn = []int{}

		for _, neighborString := range strings.Split(flagNewLifeSpawn, ",") {
			neighbor, err := strconv.Atoi(strings.TrimSpace(neighborString))

			if err != nil {
				log.Fatalf("CLI flags error: Unable to parse integer from newLifeSpawn string %s: %s", neighborString, err)
			}

			if neighbor < 1 {
				log.Fatalf("CLI flags error: Neighbor integer %d must be greater than 0", neighbor)
			}

			appNewLifeSpawn = append(appNewLifeSpawn, neighbor)
		}
	}

	if flagExistingLifeRemain != "" {
		appExistingLifeRemain = []int{}

		for _, neighborString := range strings.Split(flagExistingLifeRemain, ",") {
			neighbor, err := strconv.Atoi(strings.TrimSpace(neighborString))

			if err != nil {
				log.Fatalf("CLI flags error: Unable to parse integer from existingLifeRemain string %s: %s", neighborString, err)
			}

			if neighbor < 1 {
				log.Fatalf("CLI flags error: Neighbor integer %d must be greater than 0", neighbor)
			}

			appExistingLifeRemain = append(appExistingLifeRemain, neighbor)
		}
	}
}

// Common bootstrapping after the configuration file and CLI flags have been processed
func bootstrap() {
	if appTicks < 1 {
		log.Fatal("Bootstrap error: Number of ticks must be greater than 0:", appTicks)
	}

	// Output directory?
	if appOutputDirectory != "" {
		fileInfo, err := os.Stat(appOutputDirectory)

		if errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(appOutputDirectory, 0755)

			if err != nil {
				log.Fatalf("Bootstrap error: Unable to create output directory %s: %s", appOutputDirectory, err)
			}
		} else if !fileInfo.IsDir() {
			log.Fatalf("Bootstrap error: Output directory %s exists but is already a file", appOutputDirectory)
		}

		// Calculate the padding we'll need for naming files later
		ticksDigitsLength = 0
		i := appTicks

		for i != 0 {
			i /= 10
			ticksDigitsLength++
		}
	}

	// World dimensions sanity check
	if appWorldMinX >= appWorldMaxX {
		log.Fatalf("Bootstrap error: World X dimension minimum %d must be less than world X dimension maximum %d", appWorldMinX, appWorldMaxX)
	}

	if appWorldMinY >= appWorldMaxY {
		log.Fatalf("Bootstrap error: World Y dimension minimum %d must be less than world Y dimension maximum %d", appWorldMinY, appWorldMaxY)
	}
}

// Seeds the initial set of organisms from the input
func seedLife() map[int64]map[int64]int {
	organisms := make(map[int64]map[int64]int)

	var file io.Reader
	var err error

	if appInputFile != "" {
		if file, err = os.Open(appInputFile); err != nil {
			log.Fatal("Input error: Error opening input file")
		}
	} else {
		file = os.Stdin
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		lineLength := (len(line) - 1)

		// Left parens check
		if line[0] != 40 {
			log.Fatal("Input error: Error reading left parenthesis:", line[0])
		}

		// Right parens check
		if line[lineLength] != 41 {
			log.Fatal("Input error: Error reading right parenthesis:", line[lineLength])
		}

		// Parse coordinates
		coordinates := strings.Split(line[1:lineLength], ",")

		coordX, err := strconv.ParseInt(strings.TrimSpace(coordinates[0]), 10, 64)

		if err != nil {
			log.Fatalf("Input error: Unable to parse X-coordinate integer from input string %s: %s", coordinates[0], err)
		}

		coordY, err := strconv.ParseInt(strings.TrimSpace(coordinates[1]), 10, 64)

		if err != nil {
			log.Fatalf("Input error: Unable to parse Y-coordinate integer from input string %s: %s", coordinates[1], err)
		}

		// Check within world boundaries
		if coordX < appWorldMinX {
			log.Fatalf("Input error: X-coordinate %d outside the world minimum bounds %d", coordX, appWorldMinX)
		}

		if coordX > appWorldMaxX {
			log.Fatalf("Input error: X-coordinate %d outside the world maximum bounds %d", coordX, appWorldMaxX)
		}

		if coordY < appWorldMinY {
			log.Fatalf("Input error: Y-coordinate %d outside the world minimum bounds %d", coordY, appWorldMinY)
		}

		if coordY > appWorldMaxY {
			log.Fatalf("Input error: Y-coordinate %d outside the world maximum bounds %d", coordY, appWorldMaxY)
		}

		// Add to organisms
		addOrganism(organisms, coordX, coordY)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("Input error: Error reading standard input:", err)
	}

	return organisms
}

// Returns the neighbors to the supplied X-coordinate: left and right
// Also returns booleans marking if the left and right neighbors exist if
// wraparound is enabled
func getNeighborsX(coordX int64) (int64, int64, bool, bool) {
	var coordXLeft int64
	coordXLeftExists := true
	var coordXRight int64
	coordXRightExists := true

	if coordX == appWorldMinX {
		if appWraparound {
			coordXLeft = appWorldMaxX
		} else {
			coordXLeftExists = false
		}
	} else {
		coordXLeft = coordX - 1
	}

	if coordX == appWorldMaxX {
		if appWraparound {
			coordXRight = appWorldMinX
		} else {
			coordXRightExists = false
		}
	} else {
		coordXRight = coordX + 1
	}

	return coordXLeft, coordXRight, coordXLeftExists, coordXRightExists
}

// Returns the neighbors to the supplied Y-coordinate: bottom and top
// Also returns booleans marking if the bottom and top neighbors exist if
// wraparound is enabled
func getNeighborsY(coordY int64) (int64, int64, bool, bool) {
	var coordYBottom int64
	coordYBottomExists := true
	var coordYTop int64
	coordYTopExists := true

	if coordY == appWorldMinY {
		if appWraparound {
			coordYBottom = appWorldMaxY
		} else {
			coordYBottomExists = false
		}
	} else {
		coordYBottom = coordY - 1
	}

	if coordY == appWorldMaxY {
		if appWraparound {
			coordYTop = appWorldMinY
		} else {
			coordYTopExists = false
		}
	} else {
		coordYTop = coordY + 1
	}

	return coordYBottom, coordYTop, coordYBottomExists, coordYTopExists
}

// Adds an organism at the supplied coordinates
// Also stubs out 0 life in the organisms map in all surrounding neighbor coordinates for ease of traversal later
func addOrganism(organisms map[int64]map[int64]int, coordX int64, coordY int64) {
	coordXLeft, coordXRight, coordXLeftExists, coordXRightExists := getNeighborsX(coordX)
	coordYBottom, coordYTop, coordYBottomExists, coordYTopExists := getNeighborsY(coordY)

	coordXs := []int64{coordX}
	coordYs := []int64{coordY}

	if coordXLeftExists {
		coordXs = append(coordXs, coordXLeft)
	}

	if coordXRightExists {
		coordXs = append(coordXs, coordXRight)
	}

	if coordYBottomExists {
		coordYs = append(coordYs, coordYBottom)
	}

	if coordYTopExists {
		coordYs = append(coordYs, coordYTop)
	}

	for _, tempCoordX := range coordXs {
		if _, ok := organisms[tempCoordX]; !ok {
			organisms[tempCoordX] = make(map[int64]int)
		}

		for _, tempCoordY := range coordYs {
			if _, ok := organisms[tempCoordX][tempCoordY]; !ok {
				organisms[tempCoordX][tempCoordY] = 0
			}
		}
	}

	organisms[coordX][coordY] = 1
}

// Returns 1 for life, 0 for none for the supplied coordinates
func hasLife(organisms map[int64]map[int64]int, coordX int64, coordY int64) int {
	if coordYs, ok := organisms[coordX]; ok {
		if value, ok := coordYs[coordY]; ok {
			return value
		}
	}

	return 0
}

// Outputs the organisms in Life 1.06 format to the supplied writer; sorted too
func outputOrganisms(organisms map[int64]map[int64]int, file io.Writer) {
	coordXs := getSortedCoordXs(organisms)

	fmt.Fprintln(file, "#Life 1.06")

	for _, coordX := range coordXs {
		coordYs := getSortedCoordYs(organisms[coordX])

		for _, coordY := range coordYs {
			if organisms[coordX][coordY] == 1 {
				fmt.Fprintf(file, "%d %d\n", coordX, coordY)
			}
		}
	}
}

// Outputs the organisms for a particular tick
func outputOrganismsTick(organisms map[int64]map[int64]int, tick uint64) {
	if appOutputDirectory != "" {
		var file io.Writer
		var err error

		filename := fmt.Sprintf("%s/%0"+strconv.Itoa(int(ticksDigitsLength))+"d.txt", appOutputDirectory, tick)

		if file, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0755); err != nil {
			log.Fatalf("Error opening output file %s: %s", filename, err)
		}

		outputOrganisms(organisms, file)
	}
}

// Returns the sorted x-coordinate indices
func getSortedCoordXs(organisms map[int64]map[int64]int) []int64 {
	keys := make([]int64, 0, len(organisms))

	for k := range organisms {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(left, right int) bool {
		return keys[left] < keys[right]
	})

	return keys
}

// Returns the sorted y-coordinate indices
func getSortedCoordYs(organismsByX map[int64]int) []int64 {
	keys := make([]int64, 0, len(organismsByX))

	for k := range organismsByX {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(left, right int) bool {
		return keys[left] < keys[right]
	})

	return keys
}
