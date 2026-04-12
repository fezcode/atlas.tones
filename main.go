package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fezcode/go-piml"
)

var Version = "dev"

type Sound struct {
	Title  string `piml:"title"`
	File   string `piml:"file"`
	Type   string `piml:"type"`   // alert, ringtone
	Source string `piml:"source"` // local, url string
}

type Category struct {
	Name        string  `piml:"category"`
	Description string  `piml:"description"`
	Sounds      []Sound `piml:"sound"`
	Path        string  `piml:"-"`
}

func main() {
	setupStorage()

	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Printf("atlas.tones v%s\n", Version)
		return
	}

	categories, err := loadCategories()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading sounds: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) == 1 {
		showHelp()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "list":
		listSounds(categories)
	case "sync":
		syncRegistry()
	case "convert":
		if len(os.Args) < 3 {
			fmt.Println("Usage: atlas.tones convert <input_mp3> [output_name]")
			return
		}
		input := os.Args[2]
		output := "output.m4r"
		if len(os.Args) > 3 {
			output = os.Args[3]
			if !strings.HasSuffix(output, ".m4r") {
				output += ".m4r"
			}
		}
		convertMp3ToM4r(input, output)
	case "install":
		if len(os.Args) < 3 {
			fmt.Println("Usage: atlas.tones install <category> [sound_title]")
			return
		}
		installSound(categories, os.Args[2], getOptionalArg(3))
	default:
		showHelp()
	}
}

func setupStorage() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".atlas", "atlas.tones.data", "library")
	os.MkdirAll(path, 0755)
}

func getStoragePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".atlas", "atlas.tones.data")
}

func syncRegistry() {
	// Syncing with your fcdx.cdn repo
	url := "https://raw.githubusercontent.com/fezcode/fcdx.cdn/refs/heads/main/tones/manifest.piml"
	fmt.Printf("Syncing with FCDX CDN: %s...\n", url)
	
	dest := filepath.Join(getStoragePath(), "registry.piml")
	if err := downloadFile(url, dest); err != nil {
		fmt.Printf("Sync failed: %v\n", err)
		return
	}
	fmt.Println("Registry updated successfully!")
}

func convertMp3ToM4r(input, output string) {
	fmt.Printf("Converting %s to %s (trimmed to 30s)...\n", input, output)
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		fmt.Println("Error: 'ffmpeg' not found. Please install ffmpeg.")
		return
	}

	cmd := exec.Command("ffmpeg", "-i", input, "-t", "30", "-f", "mp4", "-c:a", "aac", "-b:a", "192k", "-ar", "44100", "-y", output)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Conversion failed: %v\n", err)
		return
	}
	fmt.Println("Successfully converted!")
}

func loadCategories() ([]Category, error) {
	var allCategories []Category

	// Global Registry (synced from CDN)
	registryPath := filepath.Join(getStoragePath(), "registry.piml")
	if _, err := os.Stat(registryPath); err == nil {
		data, _ := ioutil.ReadFile(registryPath)
		var reg struct {
			Categories []Category `piml:"category"`
		}
		if err := piml.Unmarshal(data, &reg); err == nil {
			allCategories = append(allCategories, reg.Categories...)
		}
	}

	return allCategories, nil
}

func downloadFile(url, dest string) error {
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(dest), 0755)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func installSound(categories []Category, catName, soundTitle string) {
	var targetCat *Category
	for i := range categories {
		if strings.EqualFold(categories[i].Name, catName) {
			targetCat = &categories[i]
			break
		}
	}

	if targetCat == nil {
		fmt.Printf("Error: Category '%s' not found.\n", catName)
		return
	}

	var lastPath string
	var processedSounds []string
	var isCategoryInstall bool

	if soundTitle == "" {
		isCategoryInstall = true
		fmt.Printf("Preparing all sounds from category '%s'...\n", targetCat.Name)
		for _, s := range targetCat.Sounds {
			path := processAndInstall(s)
			if path != "" {
				lastPath = path
				processedSounds = append(processedSounds, s.Title)
			}
		}
	} else {
		var targetSound *Sound
		for i := range targetCat.Sounds {
			if strings.EqualFold(targetCat.Sounds[i].Title, soundTitle) {
				targetSound = &targetCat.Sounds[i]
				break
			}
		}

		if targetSound == nil {
			fmt.Printf("Error: Sound '%s' not found in category '%s'.\n", soundTitle, catName)
			return
		}

		lastPath = processAndInstall(*targetSound)
		if lastPath != "" {
			processedSounds = append(processedSounds, targetSound.Title)
		}
	}

	// Show summary and installation instructions once at the end
	if len(processedSounds) > 0 {
		fmt.Println("\n----------------------------------------------------------------")
		if isCategoryInstall {
			fmt.Printf("Successfully prepared %d sounds from category '%s'.\n", len(processedSounds), targetCat.Name)
		} else {
			absPath, _ := filepath.Abs(lastPath)
			fmt.Printf("Target Sound: %s (%s)\n", processedSounds[0], absPath)
		}
		
		showInstructions()

		if isCategoryInstall {
			openExplorer(filepath.Dir(lastPath), true)
		} else {
			openExplorer(lastPath, false)
		}
	}
}

func processAndInstall(s Sound) string {
	// Since there are no bundled sounds, we always use the library folder
	fileName := filepath.Base(s.File)
	destPath := filepath.Join(getStoragePath(), "library", fileName)

	// URL Source handling
	if strings.HasPrefix(s.Source, "http") {
		fmt.Printf("Downloading %s...\n", s.Title)
		if err := downloadFile(s.Source, destPath); err != nil {
			fmt.Printf("Download failed: %v\n", err)
			return ""
		}
	}

	// Conversion handling
	finalPath := destPath
	if strings.HasSuffix(finalPath, ".mp3") {
		m4rPath := strings.TrimSuffix(finalPath, ".mp3") + ".m4r"
		convertMp3ToM4r(finalPath, m4rPath)
		finalPath = m4rPath
	}

	performInstall(finalPath, s.Title)
	return finalPath
}

func performInstall(absPath string, title string) {
	// Logic moved to summary in installSound
}

func showInstructions() {
	fmt.Println("HOW TO INSTALL TO IPHONE:")
	fmt.Println("1. Connect your iPhone via USB.")
	fmt.Println("2. Open iTunes (Windows) or Finder (macOS).")
	fmt.Println("3. Select your device.")
	fmt.Println("4. Drag and drop the selected file into the 'Tones' section.")
	fmt.Println("----------------------------------------------------------------")
}

func openExplorer(path string, isFolder bool) {
	absPath, _ := filepath.Abs(path)
	if runtime.GOOS == "windows" {
		if isFolder {
			exec.Command("explorer.exe", absPath).Run()
		} else {
			exec.Command("explorer.exe", "/select,", absPath).Run()
		}
	} else if runtime.GOOS == "darwin" {
		if isFolder {
			exec.Command("open", absPath).Run()
		} else {
			exec.Command("open", "-R", absPath).Run()
		}
	}
}

func listSounds(categories []Category) {
	if len(categories) == 0 {
		fmt.Println("No sounds available. Try 'atlas.tones sync' first.")
		return
	}

	for _, cat := range categories {
		fmt.Printf("\n--- %s ---\n", cat.Name)
		fmt.Printf("Description: %s\n", cat.Description)
		for _, s := range cat.Sounds {
			fmt.Printf("  > %s [%s] (%s)\n", s.Title, s.Type, s.File)
		}
	}
	fmt.Println()
}

func getOptionalArg(index int) string {
	if len(os.Args) > index {
		return os.Args[index]
	}
	return ""
}

func showHelp() {
	fmt.Println("Atlas Tones - Official iPhone Sound Catalog")
	fmt.Println("\nUsage:")
	fmt.Println("  atlas.tones list                      List all catalog sounds")
	fmt.Println("  atlas.tones sync                      Update catalog from FCDX CDN")
	fmt.Println("  atlas.tones convert <mp3> [name]      Convert MP3 to M4R (Utility)")
	fmt.Println("  atlas.tones install <cat> [sound]     Prepare catalog sound for iPhone")
}
