package main

import (
	"bufio"
	"flag"
	"fmt"
	go_bagit "github.com/nyudlts/go-bagit"
	cp "github.com/otiai10/copy"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

var (
	input       string
	bag         = "test-bag-copy"
	bagFiles    = []string{}
	uuidMatcher = regexp.MustCompile("\\b[0-9a-f]{8}\\b-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-\\b[0-9a-f]{12}\\b")
	woMatcher   = regexp.MustCompile("aspace_wo.tsv$")
	tiMatcher   = regexp.MustCompile("transfer-info.txt")
)

func init() {
	flag.StringVar(&input, "input", "", "location of bag")
}

func main() {

	fi, err := os.Stat(bag)
	if err != nil {
		//do nothing for now
	} else {
		if fi.IsDir() {
			err := os.RemoveAll(bag)
			if err != nil {
				panic(err)
			}
		}
	}

	flag.Parse()
	// resolve any symlinks
	inputPath, err := filepath.EvalSymlinks(input)
	if err != nil {
		panic(err)
	}

	//copy the bag (for dev purposes)
	err = cp.Copy(inputPath, "test-bag-copy")

	//ensure that the bag exists and is a directory
	fi, err = os.Stat(bag)
	if err != nil {
		panic(err)
	}

	if fi.IsDir() != true {
		panic(fmt.Errorf("Location provided is not a directory"))
	}

	//validate the copied bag
	if err := go_bagit.ValidateBag(bag, false, false); err != nil {
		panic(err)
	}

	//create a slice of files in the bag
	getFilesInbag()

	//find the workorder
	woPath, err := findFileInBag(woMatcher)
	if err != nil {
		panic(err)
	}

	//copy the work order to the root of the bag
	woName := filepath.Base(woPath)
	newWoLoc := filepath.Join(bag, woName)
	woBytes, err := ioutil.ReadFile(woPath)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(newWoLoc, woBytes, 0777)
	if err != nil {
		panic(err)
	}

	//get the sha256 of the work order
	wo, err := os.Open(newWoLoc)
	if err != nil {
		panic(err)
	}
	defer wo.Close()

	checksum, err := go_bagit.GenerateChecksum(wo, "sha256")
	if err != nil {
		panic(err)
	}

	//append the checksum to the tagmanifest
	tagManifest := filepath.Join(bag, "tagmanifest-sha256.txt")
	f, err := os.OpenFile(tagManifest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_, err = f.WriteString(fmt.Sprintf("%s %s\n", checksum, woName))
	if err != nil {
		panic(err)
	}

	//get the transfer-info.txt
	transferInfoPath, err := findFileInBag(tiMatcher)
	if err != nil {
		panic(err)
	}

	//append the contents of transfer-info.txt to bag-info.txt
	tiBytes, err := ioutil.ReadFile(transferInfoPath)
	if err != nil {
		panic(err)
	}
	baginfo := filepath.Join(bag, "bag-info.txt")
	b, err := os.OpenFile(baginfo, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer b.Close()
	b.Write(tiBytes)

	//append the hostname to bag-info.txt
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	b.WriteString(fmt.Sprintf("nyu-dl-hostname: %s\n", hostname))

	//append the pathname
	path, err := filepath.Abs(bag)
	if err != nil {
		panic(err)
	}
	base := filepath.Base(path)
	pathToBag := path[:len(path)-(len(base)+1)]
	b.WriteString(fmt.Sprintf("nyu-dl-pathname: %s\n", pathToBag))

	//update the tagmanifest with new baginfo sha256
	bFile, err := os.Open(baginfo)
	if err != nil {
		panic(err)
	}
	checksum, err = go_bagit.GenerateChecksum(bFile, "sha256")
	bFile.Close()
	err = updateManifest("tagmanifest-sha256.txt", "bag-info.txt", checksum)

	//validate the bag
	if err := go_bagit.ValidateBag(bag, false, false); err != nil {
		panic(err)
	}
}

func getFilesInbag() {
	err := filepath.Walk(bag, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() != true {
			bagFiles = append(bagFiles, path)
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}
}

func findFileInBag(matcher *regexp.Regexp) (string, error) {
	for _, p := range bagFiles {
		if matcher.MatchString(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("Could not locate file pattern in bag")
}

func updateManifest(manifest string, file string, checksum string) error {
	m := filepath.Join(bag, manifest)
	mFile, err := os.Open(m)
	if err != nil {
		return err
	}
	defer mFile.Close()
	scanner := bufio.NewScanner(mFile)
	lines := ""
	fileMatcher := regexp.MustCompile(file)
	for scanner.Scan() {
		if fileMatcher.MatchString(scanner.Text()) == false {
			lines = lines + scanner.Text() + "\n"
		}
	}
	lines = lines + fmt.Sprintf("%s %s\n", checksum, file)

	err = os.Remove(m)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(m, []byte(lines), 0777)
	if err != nil {
		return err
	}
	return nil
}
