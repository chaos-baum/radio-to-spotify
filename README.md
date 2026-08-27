![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/ceddicedced/radio-to-spotify/go.yml?style=for-the-badge&link=https%3A%2F%2Fgithub.com%2FCeddicedced%2Fradio-to-spotify%2Factions%2Fworkflows%2Fgo.yml)
![GitHub License](https://img.shields.io/github/license/ceddicedced/radio-to-spotify?style=for-the-badge&link=https%3A%2F%2Fgithub.com%2FCeddicedced%2Fradio-to-spotify%2Fblob%2Fmain%2FLICENSE)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ceddicedced/radio-to-spotify?style=for-the-badge)
![Docker Image Size](https://img.shields.io/docker/image-size/ceddicedced/radiotospotify?style=for-the-badge&link=https%3A%2F%2Fhub.docker.com%2Fr%2Fceddicedced%2Fradiotospotify)
# Radio to Spotify 🎵

`radio-to-spotify` is a Go-based tool that scrapes now-playing songs from online radio stations and creates Spotify playlists based on the last hour of songs played by a station. It can run as a daemon to periodically fetch and store the now-playing data.

## Quick Start
```sh
docker run -v --rm -it ceddicedced/radiotospotify
```

## Table of Contents
- [Features](#features)
- [Installation](#installation)
  - [Prerequisites](#prerequisites)
  - [Clone the Repository](#clone-the-repository)
  - [Build the Project](#build-the-project)
- [Configuration](#configuration)
  - [Station Configuration](#station-configuration)
  - [Environment Variables](#environment-variables)
- [Usage](#usage)
  - [Fetch Now Playing](#fetch-now-playing)
  - [Store Now Playing](#store-now-playing)
  - [Create Spotify Playlist](#create-spotify-playlist)
  - [Run as a Daemon](#run-as-a-daemon)
- [Running with Docker](#running-with-docker)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgements](#acknowledgements)
- [Contact](#contact)

## Features
- **Fetch Now-Playing Songs**: Scrape current songs playing on various radio stations.
- **Store Fetched Songs**: Store songs in a file, SQLite, or PostgreSQL database.
- **Create Spotify Playlists**: Generate Spotify playlists based on stored songs.
- **Run as a Daemon**: Periodically fetch and store songs.

## Installation

### Prerequisites
- [Go](https://golang.org/doc/install) (version 1.16+)
- [SQLite](https://www.sqlite.org/index.html) (if using SQLite storage)
- [PostgreSQL](https://www.postgresql.org/) (if using PostgreSQL storage)
- [Spotify Developer Account](https://developer.spotify.com/)

### Clone the Repository
```sh
git clone https://github.com/ceddicedced/radio-to-spotify.git
cd radio-to-spotify
```

### Build the Project
```sh
go build -o radio-to-spotify main.go
```

## Configuration

### Station Configuration
Create a `stations.json` file to define the radio stations to scrape:

```json
{
  [
    {
      "id": "1live",
      "name": "WDR 1LIVE",
      "url": "https://www.wdr.de/radio/radiotext/streamtitle_1live.txt",
      "type": "plaintext",
      "regex": "(?P<artist>.+?) - (?P<title>.+)",
      "playlistID": "3yJEj5bAmhc6bFREajJu6t"
    },
    {
      "id": "rs2",
      "name": "94.3 rs2",
      "url": "https://iris-rs2.loverad.io/flow.json?station=7",
      "type": "json",
      "artistKey": ["result", "entry", 0, "song", "entry", 0, "artist", "entry", 0, "name"],
      "titleKey": ["result", "entry", 0, "song", "entry", 0, "title"],
      "playlistID" :"4v8AVb5aOixKbR2NjDZuLq"
    },
    {
      "id": "radio1",
      "name": "Radio Eins",
      "url": "https://www.radioeins.de/include/rad/nowonair/now_on_air.html",
      "type": "html",
      "artistTag": "p.artist",
      "titleTag": "p.songtitle",
      "playlistID": "2S41FhYMM5TDT3bL8XYWEK"
    }
  ]
}
```
### Station Configuration Fields
- `id`: Unique identifier for the station.
- `name`: Name of the station.
- `url`: URL to scrape the now-playing songs.
- `type`: Type of response (html or json or plaintext).
- `artistTag`: HTML tag for the artist name (html type).
- `titleTag`: HTML tag for the song title (html type).
- `artistKey`: JSON key for the artist name (json type).
- `regex`: Regular expression to extract the artist and title (plaintext type).
- `titleKey`: JSON key for the song title (json type).
- `playlistID`: Spotify playlist ID to add the songs.


### Environment Variables
Set up your environment variables for Spotify integration:
- `SPOTIFY_ID`: Your Spotify Client ID
- `SPOTIFY_SECRET`: Your Spotify Client Secret
- `SPOTIFY_REDIRECT_URL`: Your Spotify Redirect URL (must start with `https://`)

You can store these in a `.env` file:
```sh
SPOTIFY_ID=your_spotify_client_id
SPOTIFY_SECRET=your_spotify_client_secret
SPOTIFY_REDIRECT_URL=https://your-domain.com/callback
```
## Usage

### Fetch Now Playing
Fetch the now-playing songs for all stations defined in the `stations.json` file:
```sh
./radio-to-spotify fetch --config=stations.json --loglevel=debug --storage=file --storage-path=data/db.json
```

### Store Now Playing
Fetch and store now-playing songs for a specific station (dry run):
```sh
./radio-to-spotify store --config=stations.json --station=radiofritz --loglevel=info --dry-run --storage=file --storage-path=data/db.json
```

### Create Spotify Playlist
Create a Spotify playlist for the last hour of songs for a specific station:
```sh
./radio-to-spotify playlist --config=stations.json --station=radiofritz --loglevel=error --storage=file --storage-path=data/db.json --playlist-range=lasthour
```

### Run as a Daemon
Run the tool as a daemon to periodically fetch and store now-playing songs:
```sh
./radio-to-spotify daemon --config=stations.json --loglevel=debug --storage=file --storage-path=data/db.json --interval=1m --playlist-range=lasthour
```

## Running with Docker
You can use the provided Docker image `ceddicedced/radiotospotify` to run the application:

```sh
docker pull ceddicedced/radiotospotify
docker run -v $(pwd)/data:/app/data -e SPOTIFY_ID -e SPOTIFY_SECRET -e SPOTIFY_REDIRECT_URL ceddicedced/radiotospotify daemon --config=stations.json --loglevel=debug --storage=file --storage-path=data/db.json --interval=1m --playlist-range=lasthour
```

To build and run your own Docker image:
```sh
docker build -t radio-to-spotify .
docker run -v $(pwd)/data:/app/data -e SPOTIFY_ID -e SPOTIFY_SECRET -e SPOTIFY_REDIRECT_URL radio-to-spotify daemon --config=stations.json --loglevel=debug --storage=file --storage-path=data/db.json --interval=1m --playlist-range=lasthour
```

## Contributing
Contributions are welcome! Please open an issue or submit a pull request for any changes.

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgements
- [Go](https://golang.org/)
- [Cobra](https://github.com/spf13/cobra)
- [Logrus](https://github.com/sirupsen/logrus)
- [Spotify Web API](https://developer.spotify.com/documentation/web-api/)

## Contact
Created by [Ceddicedced](https://github.com/ceddicedced) - feel free to contact me!