# README

This is a Go-implemented simulation of [Conway's Game of Life](https://conwaylife.com/wiki/Conway%27s_Game_of_Life).

## Configuration File

The configuration file, which can be specified by a CLI flag (below), configures
the same things which the CLI flags configure. Those options are detailed below.
A sample configuration file can be seen at `configuration.dist.yaml`.

## CLI Flags

CLI flags override anything loaded via the configuration file.

- `-configuration <file>`: Use a configuration file
- `-help`: Shows the help/usage of the application
- `-input <file>`: Rather than use `stdin` for the input to the application, you
  can have it use this file instead
- `-output <file>`: Rather than use `stdout` for the Life 1.06 output of the application,
  you can have it use this file instead
- `-outdir <directory>`: If you want to output the Life 1.06 format for **every
  single tick** of the world, you can set this to a directory and an output file
  will be generated for each tick, including the seed (which will be `*0.txt`)
- `-ticks <integer>`: The number of ticks/iterations to run in the world
- `-nowrap`: By default, the world will wrap at the edges to the other side;
  you can disable this behavior if you want the world to have hard boundaries at
  the edges
- `-world <dimensions>`: By default the world will be as large as the signed 64-bit
  integer allows; you can set this manually with the format `min-x:max-x;min-y:max-y`
- `-newlife <csv>`: With comma-delimited integers, you can set when new life will
  spawn in a coordinate
- `-exlife <csv>`: With comma-delimited integers, you can set when existing life
  will remain in a coordinate

## Organisms Map

The organisms map is indexed by the x-coordinate first, then the y-coordinate, mapped
to an integer notating alive or dead. Example illustration in JSON:

```json
{
  "x_coordinate_1": {
    "y_coordinate_1": 1,
    "y_coordinate_2": 0
  }
}
```

When adding new live organisms to the map, additional entries for all 8 neighbors
will be added and set to 0 (if they are not present/alive already) to make iterating
over the map for the next generation of life easier. No entries are in the map if
they are either not alive or a neighbor to a live organism - there's no need to
keep track of coordinates which cannot be alive in the next tick of the world.
