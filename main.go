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
	"github.com/pterm/pterm"
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
		pterm.Info.Printf("atlas.tones v%s\n", Version)
		return
	}

	categories, err := loadCategories()
	if err != nil {
		pterm.Error.Printf("Error loading sounds: %v\n", err)
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
			pterm.Warning.Println("Usage: atlas.tones convert <input_mp3> [output_name]")
			return
		}
		if !ensureFFmpeg() {
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
			pterm.Warning.Println("Usage: atlas.tones install <category> [sound_title]")
			return
		}
		if !ensureFFmpeg() {
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
	url := "https://raw.githubusercontent.com/fezcode/fcdx.cdn/refs/heads/main/tones/manifest.piml"
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Syncing with FCDX CDN: %s...", url))
	
	dest := filepath.Join(getStoragePath(), "registry.piml")
	if err := downloadFile(url, dest); err != nil {
		spinner.Fail(fmt.Sprintf("Sync failed: %v", err))
		return
	}
	spinner.Success("Registry updated successfully!")
}

func getFFmpegPath() string {
	path, err := exec.LookPath("ffmpeg")
	if err == nil {
		return path
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join(getStoragePath(), "ffmpeg" + ext)
}

func ensureFFmpeg() bool {
	path, err := exec.LookPath("ffmpeg")
	if err == nil {
		pterm.Success.Printf("FFmpeg found in PATH: %s\n", path)
		return true
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	localPath := filepath.Join(getStoragePath(), "ffmpeg" + ext)
	if _, err := os.Stat(localPath); err == nil {
		pterm.Success.Printf("FFmpeg found locally: %s\n", localPath)
		return true
	}

	pterm.Warning.Println("FFmpeg is required for MP3 to M4R conversion but was not found.")
	
	result, _ := pterm.DefaultInteractiveConfirm.WithDefaultText("Would you like to download a standalone FFmpeg binary now?").Show()
	if result {
		err := downloadFFmpeg(localPath)
		if err != nil {
			pterm.Error.Printf("Failed to download FFmpeg: %v\n", err)
			pterm.Info.Println("Please download FFmpeg manually from https://ffmpeg.org/download.html and add it to your PATH.")
			return false
		}
		pterm.Success.Printf("FFmpeg downloaded successfully to %s\n", localPath)
		return true
	}

	pterm.Info.Println("Please download FFmpeg manually from https://ffmpeg.org/download.html and add it to your PATH to use the conversion feature.")
	return false
}

func getFFmpegDownloadURL() (string, error) {
	baseURL := "https://github.com/eugeneware/ffmpeg-static/releases/download/b4.4/"
	
	var assetName string
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "amd64" {
			assetName = "win32-x64"
		} else if runtime.GOARCH == "386" {
			assetName = "win32-ia32"
		}
	case "darwin":
		if runtime.GOARCH == "amd64" {
			assetName = "darwin-x64"
		} else if runtime.GOARCH == "arm64" {
			assetName = "darwin-arm64"
		}
	case "linux":
		if runtime.GOARCH == "amd64" {
			assetName = "linux-x64"
		} else if runtime.GOARCH == "arm64" {
			assetName = "linux-arm64"
		} else if runtime.GOARCH == "arm" {
			assetName = "linux-arm"
		} else if runtime.GOARCH == "386" {
			assetName = "linux-ia32"
		}
	}

	if assetName == "" {
		return "", fmt.Errorf("unsupported OS/Arch combination: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return baseURL + assetName, nil
}

func downloadFFmpeg(destPath string) error {
	url, err := getFFmpegDownloadURL()
	if err != nil {
		return err
	}

	spinner, _ := pterm.DefaultSpinner.Start("Downloading FFmpeg binary (~20-30MB)...")
	
	os.MkdirAll(filepath.Dir(destPath), 0755)
	
	resp, err := http.Get(url)
	if err != nil {
		spinner.Fail()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		spinner.Fail()
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		spinner.Fail()
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		spinner.Fail()
		return err
	}

	if runtime.GOOS != "windows" {
		os.Chmod(destPath, 0755)
	}

	spinner.Success()
	return nil
}

func convertMp3ToM4r(input, output string) {
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Converting %s to %s (trimmed to 30s)...", input, output))

	ffmpegPath := getFFmpegPath()
	if ffmpegPath == "" {
		spinner.Fail("Error: 'ffmpeg' not found.")
		return
	}

	cmd := exec.Command(ffmpegPath, "-i", input, "-t", "30", "-f", "mp4", "-c:a", "aac", "-b:a", "192k", "-ar", "44100", "-y", output)
	if err := cmd.Run(); err != nil {
		spinner.Fail(fmt.Sprintf("Conversion failed: %v", err))
		return
	}
	spinner.Success("Successfully converted!")
}

func loadCategories() ([]Category, error) {
	var allCategories []Category

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
		pterm.Error.Printf("Category '%s' not found.\n", catName)
		return
	}

	var lastPath string
	var processedSounds []string
	var isCategoryInstall bool

	if soundTitle == "" {
		isCategoryInstall = true
		pterm.Info.Printf("Preparing all sounds from category '%s'...\n", targetCat.Name)
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
			pterm.Error.Printf("Sound '%s' not found in category '%s'.\n", soundTitle, catName)
			return
		}

		lastPath = processAndInstall(*targetSound)
		if lastPath != "" {
			processedSounds = append(processedSounds, targetSound.Title)
		}
	}

	if len(processedSounds) > 0 {
		fmt.Println()
		pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue)).WithTextStyle(pterm.NewStyle(pterm.FgBlack)).Println("INSTALLATION SUCCESS")
		
		if isCategoryInstall {
			pterm.Success.Printf("Successfully prepared %d sounds from category '%s'.\n", len(processedSounds), targetCat.Name)
		} else {
			absPath, _ := filepath.Abs(lastPath)
			pterm.Success.Printf("Target Sound: %s (%s)\n", processedSounds[0], absPath)
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
	fileName := filepath.Base(s.File)
	destPath := filepath.Join(getStoragePath(), "library", fileName)

	if strings.HasPrefix(s.Source, "http") {
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Downloading %s...", s.Title))
		if err := downloadFile(s.Source, destPath); err != nil {
			spinner.Fail(fmt.Sprintf("Download failed: %v", err))
			return ""
		}
		spinner.Success(fmt.Sprintf("Downloaded %s", s.Title))
	}

	finalPath := destPath
	if strings.HasSuffix(finalPath, ".mp3") {
		m4rPath := strings.TrimSuffix(finalPath, ".mp3") + ".m4r"
		
		spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Converting %s to M4R...", s.Title))
		
		ffmpegPath := getFFmpegPath()
		if ffmpegPath == "" {
			spinner.Fail("Error: 'ffmpeg' not found.")
			return "" // conversion failed
		}

		cmd := exec.Command(ffmpegPath, "-i", finalPath, "-t", "30", "-f", "mp4", "-c:a", "aac", "-b:a", "192k", "-ar", "44100", "-y", m4rPath)
		if err := cmd.Run(); err != nil {
			spinner.Fail(fmt.Sprintf("Conversion failed: %v", err))
			return "" // conversion failed
		}
		spinner.Success(fmt.Sprintf("Converted %s to M4R", s.Title))
		finalPath = m4rPath
	}

	performInstall(finalPath, s.Title)
	return finalPath
}

func performInstall(absPath string, title string) {
	// Logic moved to summary in installSound
}

func showInstructions() {
	panels := pterm.Panels{
		{{Data: pterm.DefaultBox.WithTitle("HOW TO INSTALL TO IPHONE").Sprint(
			"1. Connect your iPhone via USB.\n" +
			"2. Open iTunes (Windows) or Finder (macOS).\n" +
			"3. Select your device.\n" +
			"4. Drag and drop the selected file(s) into the 'Tones' section.",
		)}},
	}
	pterm.DefaultPanel.WithPanels(panels).Render()
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
		pterm.Warning.Println("No sounds available. Try 'atlas.tones sync' first.")
		return
	}

	pterm.DefaultHeader.WithFullWidth().Println("Atlas Tones Catalog")

	for _, cat := range categories {
		pterm.DefaultSection.Println(cat.Name)
		pterm.Info.Println(cat.Description)
		
		tableData := pterm.TableData{{"Title", "Type", "File"}}
		for _, s := range cat.Sounds {
			tableData = append(tableData, []string{s.Title, s.Type, filepath.Base(s.File)})
		}
		pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
	}
}

func getOptionalArg(index int) string {
	if len(os.Args) > index {
		return os.Args[index]
	}
	return ""
}

func showHelp() {
	pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).WithTextStyle(pterm.NewStyle(pterm.FgBlack)).Println("Atlas Tones - Official iPhone Sound Catalog")
	
	pterm.Info.Println("A CLI tool to download and prepare ringtones for your iPhone.")
	fmt.Println()
	
	tableData := pterm.TableData{
		{"Command", "Description"},
		{"atlas.tones list", "List all catalog sounds"},
		{"atlas.tones sync", "Update catalog from FCDX CDN"},
		{"atlas.tones convert <mp3> [name]", "Convert MP3 to M4R (Utility)"},
		{"atlas.tones install <cat> [sound]", "Prepare catalog sound for iPhone"},
	}
	
	pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
}
