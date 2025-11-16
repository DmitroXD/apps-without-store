package main

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "strings"
    "log"
    "time"
    "runtime"

    "github.com/PuerkitoBio/goquery"
)

const storeUrl = "https://store.rg-adguard.net/api/GetFiles"
const token = "9NKSQGP7F2NH"
const microsoftUrl = "https://www.microsoft.com/store/apps"
const appUrl = microsoftUrl + "/" + token

var client = http.Client{}
var logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

type Item struct {
    Name   string
    Url    string
    Expire string
    Sha1   string
}

type Loader struct {
    Appx        []Item
    Msixbundle  []Item
}

func detectArch() string {
    switch runtime.GOARCH {
    case "amd64":
        return "x64"
    case "386":
        return "x86"
    case "arm64":
        return "arm64"
    default:
        return ""
    }
}

func (l *Loader) Add(item Item) {
    name := strings.ToLower(item.Name)

    if strings.HasSuffix(name, ".appx") {
        l.Appx = append(l.Appx, item)
    }
    if strings.HasSuffix(name, ".msixbundle") {
        l.Msixbundle = append(l.Msixbundle, item)
    }
}

func extractToken(url string) string {
    parts := strings.Split(url, "/")
    return parts[len(parts)-1]
}


func downloadFile(item Item) (string, error) {
    logger.Printf("Starting download for %s\n", item.Name)

    tempDir := "./downloads"
    if err := os.MkdirAll(tempDir, 0755); err != nil {
        logger.Printf("Dir error: %v\n", err)
        return "", err
    }

    filePath := tempDir + "/" + item.Name
    resp, err := http.Get(item.Url)
    if err != nil {
        logger.Printf("Download error: %v\n", err)
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("received non-OK HTTP status %d", resp.StatusCode)
    }

    file, err := os.Create(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    if _, err = io.Copy(file, resp.Body); err != nil {
        return "", err
    }

    logger.Printf("Downloaded: %s\n", filePath)
    return filePath, nil
}

func installFile(item Item) error {
    fmt.Printf("Installing %s\n", item.Name)

    filePath, err := downloadFile(item)
    if err != nil {
        return err
    }

    cmd := exec.Command("powershell", "-Command", fmt.Sprintf("Add-AppxPackage -Path \"%s\"", filePath))
    var output strings.Builder
    cmd.Stdout = &output
    cmd.Stderr = &output

    if err := cmd.Run(); err != nil {
        fmt.Printf("Install error: %s\n", output.String())
        return err
    }

    fmt.Printf("Installed %s\n", item.Name)
    return nil
}

func (l *Loader) InstallDependencies(arch string) {
    for _, item := range l.Appx {
        if strings.Contains(strings.ToLower(item.Name), arch) {
            installFile(item)
        }
    }
}

func (l *Loader) InstallProgram() {
    for _, item := range l.Msixbundle {
        installFile(item)
    }
}

func main() {
    arch := detectArch()
    fmt.Printf("Detected architecture: %s\n", arch)

    fmt.Print("Enter Microsoft Store app URL: ")
    var userUrl string
    fmt.Scanln(&userUrl)

    if !strings.Contains(userUrl, "microsoft.com") {
        fmt.Println("Invalid URL. Must be a Microsoft Store link.")
        return
    }

    resp, err := client.PostForm(storeUrl, map[string][]string{
        "type": {"url"},
        "url":  {userUrl},
    })
    if err != nil {
        fmt.Println("Request error:", err)
        return
    }
    defer resp.Body.Close()

    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        fmt.Println("HTML parse error:", err)
        return
    }

    var loader Loader
    doc.Find("table.tftable tr").Each(func(index int, row *goquery.Selection) {
        if index == 0 {
            return
        }

        cells := row.Find("td")
        name := cells.Eq(0).Text()
        url, _ := cells.Eq(0).Find("a").Attr("href")
        expire := cells.Eq(1).Text()
        sha1 := cells.Eq(2).Text()

        item := Item{
            Name:   name,
            Url:    url,
            Expire: expire,
            Sha1:   sha1,
        }

        loader.Add(item)
    })

    loader.InstallDependencies(arch)
    loader.InstallProgram()

    fmt.Printf("Finish\n")
    time.Sleep(5 * time.Second)
}

