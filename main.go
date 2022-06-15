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
)

// Prepare all CLI flags, as well as global variables derived
var (
	flagShowHelp        bool
	flagInputFile       string
	flagOutputFile      string
	flagOutputDirectory string
	flagTicks           uint64
	flagWraparound      bool
	flagWorldDimensions string

	worldMinX int64
	worldMaxX int64
	worldMinY int64
	worldMaxY int64

	ticksDigitsLength uint64
)

// Init flags system
func init() {
	flag.BoolVar(&flagShowHelp, "help", false, "Show help")
	flag.StringVar(&flagInputFile, "input", "", "Input file to use rather than stdin")
	flag.StringVar(&flagOutputFile, "output", "", "Output file to use rather than stdout")
	flag.StringVar(&flagOutputDirectory, "outdir", "", "Output directory to log all ticks")
	flag.Uint64Var(&flagTicks, "ticks", 10, "Number of ticks to run")
	flag.BoolVar(&flagWraparound, "wraparound", true, "Wrap the world around at the edges")
	flag.StringVar(&flagWorldDimensions, "world", fmt.Sprintf("%d:%d;%d:%d", math.MinInt64, math.MaxInt64, math.MinInt64, math.MaxInt64), "The dimensions of the world in format min-x:max-x;min-y:max-y")
}

func main() {
	// Bootstrap
	bootstrap()

	// Seed life
	organisms := seedLife()

	// Run the simulation
	var tick uint64

	for tick = 0; tick < flagTicks; tick++ {
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

				// Coordinates will be alive if:
				// 1. Already alive and 2-3 live neighbors
				// 2. Not alive and 3 live neighbors
				if neighborsAlive >= 2 && neighborsAlive <= 3 {
					if alive == 1 || neighborsAlive == 3 {
						addOrganism(organismsNext, coordX, coordY)
					}
				}
			}
		}

		organisms = organismsNext
	}

	// And we're done; let's wrap up
	outputOrganismsTick(organisms, flagTicks)

	var file io.Writer
	var err error

	if flagOutputFile != "" {
		if file, err = os.OpenFile(flagOutputFile, os.O_RDWR|os.O_CREATE, 0755); err != nil {
			log.Fatal("Error opening output file:", flagOutputFile)
		}
	} else {
		file = os.Stdout
	}

	outputOrganisms(organisms, file)
	os.Exit(0)
}

// Parses CLI flags and handles any upfront processing resulting from said flags
func bootstrap() {
	// CLI flags
	flag.Parse()

	if flagShowHelp {
		flag.Usage()

		os.Exit(0)
	}

	if flagTicks < 1 {
		log.Fatal("Number of ticks must be greater than 0:", flagTicks)
	}

	// Setup world dimensions
	parts := strings.Split(flagWorldDimensions, ";")
	numberParts := len(parts)

	if numberParts != 2 {
		log.Fatal("Incorrect number of dimensions in world:", numberParts)
	}

	partsX := strings.Split(parts[0], ":")
	numberPartsX := len(partsX)

	if numberPartsX != 2 {
		log.Fatal("Incorrect number of directions in X dimension:", numberPartsX)
	}

	partsY := strings.Split(parts[1], ":")
	numberPartsY := len(partsY)

	if numberPartsY != 2 {
		log.Fatal("Incorrect number of directions in X dimension:", numberPartsY)
	}

	var err error

	worldMinX, err = strconv.ParseInt(strings.TrimSpace(partsX[0]), 10, 64)

	if err != nil {
		log.Fatalf("Unable to parse world X dimension minimum %s: %s", partsX[0], err)
	}

	worldMaxX, err = strconv.ParseInt(strings.TrimSpace(partsX[1]), 10, 64)

	if err != nil {
		log.Fatalf("Unable to parse world X dimension maximum %s: %s", partsX[1], err)
	}

	worldMinY, err = strconv.ParseInt(strings.TrimSpace(partsY[0]), 10, 64)

	if err != nil {
		log.Fatalf("Unable to parse world Y dimension minimum %s: %s", partsY[0], err)
	}

	worldMaxY, err = strconv.ParseInt(strings.TrimSpace(partsY[1]), 10, 64)

	if err != nil {
		log.Fatalf("Unable to parse world Y dimension maximum %s: %s", partsY[1], err)
	}

	// Output directory?
	if flagOutputDirectory != "" {
		fileInfo, err := os.Stat(flagOutputDirectory)

		if errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(flagOutputDirectory, 0755)

			if err != nil {
				log.Fatalf("Unable to create output directory %s: %s", flagOutputDirectory, err)
			}
		} else if !fileInfo.IsDir() {
			log.Fatalf("Output directory %s exists but is already a file", flagOutputDirectory)
		}

		// Calculate the padding we'll need for naming files later
		ticksDigitsLength = 0
		i := flagTicks

		for i != 0 {
			i /= 10
			ticksDigitsLength++
		}
	}
}

// Seeds the initial set of organisms from the input
func seedLife() map[int64]map[int64]int {
	organisms := make(map[int64]map[int64]int)

	var file io.Reader
	var err error

	if flagInputFile != "" {
		if file, err = os.Open(flagInputFile); err != nil {
			log.Fatal("Error opening input file")
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
			log.Fatal("Error reading left parenthesis:", line[0])
		}

		// Right parens check
		if line[lineLength] != 41 {
			log.Fatal("Error reading right parenthesis:", line[lineLength])
		}

		// Parse coordinates
		coordinates := strings.Split(line[1:lineLength], ",")

		coordX, err := strconv.ParseInt(strings.TrimSpace(coordinates[0]), 10, 64)

		if err != nil {
			log.Fatalf("Unable to parse X-coordinate integer from stdin string %s: %s", coordinates[0], err)
		}

		coordY, err := strconv.ParseInt(strings.TrimSpace(coordinates[1]), 10, 64)

		if err != nil {
			log.Fatalf("Unable to parse Y-coordinate integer from stdin string %s: %s", coordinates[1], err)
		}

		// Check within world boundaries
		if coordX < worldMinX {
			log.Fatalf("X-coordinate %d outside the world minimum bounds %d", coordX, worldMinX)
		}

		if coordX > worldMaxX {
			log.Fatalf("X-coordinate %d outside the world maximum bounds %d", coordX, worldMaxX)
		}

		if coordY < worldMinY {
			log.Fatalf("Y-coordinate %d outside the world minimum bounds %d", coordY, worldMinY)
		}

		if coordY > worldMaxY {
			log.Fatalf("Y-coordinate %d outside the world maximum bounds %d", coordY, worldMaxY)
		}

		// Add to organisms
		addOrganism(organisms, coordX, coordY)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("Error reading standard input:", err)
	}

	return organisms
}

// Returns the neighbors to the supplied X-coordinate: left and right
// Also returns boolean flags as to if the left and right neighbors exist if
// wraparound is enabled
func getNeighborsX(coordX int64) (int64, int64, bool, bool) {
	var coordXLeft int64
	coordXLeftExists := true
	var coordXRight int64
	coordXRightExists := true

	if coordX == worldMinX {
		if flagWraparound {
			coordXLeft = worldMaxX
		} else {
			coordXLeftExists = false
		}
	} else {
		coordXLeft = coordX - 1
	}

	if coordX == worldMaxX {
		if flagWraparound {
			coordXRight = worldMinX
		} else {
			coordXRightExists = false
		}
	} else {
		coordXRight = coordX + 1
	}

	return coordXLeft, coordXRight, coordXLeftExists, coordXRightExists
}

// Returns the neighbors to the supplied Y-coordinate: bottom and top
// Also returns boolean flags as to if the bottom and top neighbors exist if
// wraparound is enabled
func getNeighborsY(coordY int64) (int64, int64, bool, bool) {
	var coordYBottom int64
	coordYBottomExists := true
	var coordYTop int64
	coordYTopExists := true

	if coordY == worldMinY {
		if flagWraparound {
			coordYBottom = worldMaxY
		} else {
			coordYBottomExists = false
		}
	} else {
		coordYBottom = coordY - 1
	}

	if coordY == worldMaxY {
		if flagWraparound {
			coordYTop = worldMinY
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
	if flagOutputDirectory != "" {
		var file io.Writer
		var err error

		filename := fmt.Sprintf("%s/%0"+strconv.Itoa(int(ticksDigitsLength))+"d.txt", flagOutputDirectory, tick)

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
