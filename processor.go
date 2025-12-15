// This is a post-processor script meant to be run after the Mintlify OpenAPI scraper.
// It helps to populate the API reference navigation structure in the most organized way possible.
//
// Tip: run this via `make generate`.
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The directory where the Mintlify scraper files are stored
var tmpDir = "tmp"

// The directory where the files are stored that are already organized and therefore should be removed from this tmp folder
var targetDir = "reference/api"

// Files in the targetDir that should be ignored (not removed) even though they do not appear in the OpenAPI spec
var ignoreFiles = map[string]bool{
	fmt.Sprintf("%s/introduction.mdx", targetDir): true,
}

func main() {

	defer clean(tmpDir)
	existingFiles, newFiles, err := match(tmpDir, targetDir)
	if err != nil {
		fmt.Println(err)
		return
	}

	oldFiles, err := getAllFilesInDir(targetDir)
	if err != nil {
		fmt.Println(err)
		return
	}

	var removeFiles []string
	for _, oldFile := range oldFiles {
		if ok, _ := ignoreFiles[oldFile]; ok {
			continue
		}

		found := false
		for _, existingFile := range existingFiles {
			if oldFile == existingFile {
				found = true
				break
			}
		}
		if !found {
			removeFiles = append(removeFiles, oldFile)
		}
	}

	if len(removeFiles) > 0 {
		fmt.Println("=====================================")
		fmt.Println("The following files are no longer in the OpenAPI spec and will be removed:")
		for _, file := range removeFiles {
			fmt.Println(file)
		}
		fmt.Println("Do you want to proceed? (y/n)")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}
		answer = strings.TrimSpace(answer)
		if answer != "y" {
			fmt.Println("Aborted.")
			return
		}

		for _, file := range removeFiles {
			err := os.Remove(file)
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}

	if len(newFiles) == 0 {
		fmt.Println("Done.")
		return
	}

	fmt.Println("=====================================")
	fmt.Println("One or more new API endpoints were found!")
	fmt.Println("Please state what these API endpoints should appear as in the API reference sidebar.")
	fmt.Println("(Leave empty to accept the default suggestion)")

	renamedFiles, err := rename(tmpDir, targetDir, newFiles)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Now add the newly generated files to mint.json (create your own groups!):")
	sort.Sort(sort.StringSlice(renamedFiles))
	for _, file := range renamedFiles {
		fmt.Printf("\"%s\",\n", strings.TrimSuffix(file, ".mdx"))
	}
}

// match traverses the `tmp` directory already exist in the `reference` directory and which ones are new.
func match(tmpDir string, targetDir string) (existingFiles []string, newFiles []string, err error) {
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			matchingFile, err := findMatchingFile(targetDir, content)
			if err != nil {
				return err
			}

			if matchingFile != "" {
				err := os.Remove(path)
				if err != nil {
					return err
				}
				existingFiles = append(existingFiles, matchingFile)
			} else {
				newFiles = append(newFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return existingFiles, newFiles, nil
}

func clean(tmpDir string) {
	_ = os.RemoveAll(fmt.Sprintf("%s/", tmpDir))
	return
}

func getAllFilesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// rename renames the files in the tmp directory to the names that the user specifies.
// This affects how the API endpoints are displayed in the API reference sidebar.
func rename(tmpDir string, targetDir string, files []string) (newfiles []string, err error) {
	reader := bufio.NewReader(os.Stdin)

	for _, oldPath := range files {
		file, err := os.Open(oldPath)
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(file)
		scanner.Scan() // read and discard the first line
		scanner.Scan() // read the second line
		route := strings.TrimPrefix(scanner.Text(), "openapi: ")
		err = file.Close()
		if err != nil {
			return nil, err
		}

		spl := strings.SplitN(route, " ", 2)
		if len(spl) < 2 {
			return nil, fmt.Errorf("unexpected route name format: %s", route)
		}

		isPlural := false
		var sb strings.Builder
		switch spl[0] {
		case "get":
			if strings.Contains(spl[1], "{") {
				sb.WriteString("Get ")
			} else if strings.Contains(spl[1], "/batch/") {
				isPlural = true
				sb.WriteString("Batch Get ")
			} else {
				isPlural = true
				sb.WriteString("List ")
			}
		case "post":
			sb.WriteString("Create ")
		case "put", "patch":
			sb.WriteString("Update ")
		case "delete":
			sb.WriteString("Delete ")
		}

		pathSpl := strings.Split(spl[1], "/")
		if len(pathSpl) < 3 {
			return nil, fmt.Errorf("unexpected route name format: %s", route)
		}

		// Always assume a structure like ["", "api", "..."]
		resource := pathSpl[2]
		targetDirectory := fmt.Sprintf("%s/%s/", targetDir, strings.Join(pathSpl[2:len(pathSpl)], "/"))
		targetDirectory = strings.ReplaceAll(targetDirectory, "_", "-")

		resource = strings.ReplaceAll(resource, "-", " ")
		resource = strings.ReplaceAll(resource, "_", " ")
		resource = strings.Title(resource)
		if !isPlural {
			resource = strings.TrimSuffix(resource, "s")
		}
		sb.WriteString(resource)

		defaultName := sb.String()

		fmt.Printf("Route: \"%s\" (default: \"%s\") => ", route, defaultName)
		newName, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		newName = strings.TrimSpace(newName)

		if newName == "" {
			newName = defaultName
		}

		newName = strings.ReplaceAll(newName, " ", "-")
		newName = strings.ToLower(newName)
		newName = fmt.Sprintf("%s.mdx", newName)
		newPath := filepath.Join(filepath.Dir(targetDirectory), newName)

		if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
			err = os.MkdirAll(targetDirectory, 0755)
			if err != nil {
				return nil, err
			}
		}
		err = os.Rename(oldPath, newPath)
		if err != nil {
			return nil, err
		}
		newfiles = append(newfiles, newPath)
	}

	return newfiles, nil
}

// findMatchingFile checks if a file with the same content as the given content exists in the given directory or its child directories.
// Returns the path of the file that was found to match, or an empty string if it does not exist.
func findMatchingFile(dir string, content []byte) (res string, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileContent, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			if string(fileContent) == string(content) {
				res = path
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return res, nil
}
