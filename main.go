package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var logger *log.Logger

func main() {
	// Открытие лог-файла для записи
	logFile, err := os.OpenFile("process.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Ошибка при открытии лог-файла: %v", err)
	}
	defer logFile.Close()

	// Настройка логгера
	logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)

	// Чтение репозиториев из файла
	repos, err := readReposFromFile("repos.txt")
	if err != nil {
		log.Fatalf("Ошибка при чтении файла repos.txt: %v", err)
	}

	// Вывод количества репозиториев
	logger.Printf("Чтение %d ссылок из файла repos.txt", len(repos))

	if len(repos) == 0 {
		log.Fatal("Файл repos.txt пуст или не содержит валидных ссылок")
	}

	// Обработка каждого репозитория
	for _, repoURL := range repos {
		fmt.Printf("\n🔧 Обработка репозитория: %s\n", repoURL)
		logger.Printf("Обработка репозитория: %s", repoURL)
		processRepository(repoURL)
	}
}

func readReposFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var repos []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			repos = append(repos, line)
		}
	}
	return repos, scanner.Err()
}

func processRepository(repoURL string) {
	tmpDir, err := os.MkdirTemp("", "git-tags-*")
	if err != nil {
		logger.Printf("Не удалось создать временную директорию: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Клонируем репозиторий
	fmt.Println("Клонируем репозиторий...")
	logger.Printf("Клонирование репозитория %s", repoURL)
	cmd := exec.Command("git", "clone", "--quiet", repoURL, tmpDir)
	if err := cmd.Run(); err != nil {
		logger.Printf("Ошибка при клонировании репозитория %s: %v\n", repoURL, err)
		return
	}

	// Получаем список тегов
	cmd = exec.Command("git", "for-each-ref",
		"--sort=creatordate",
		"--format=%(refname:short)|%(creatordate:iso8601)",
		"refs/tags")
	cmd.Dir = tmpDir
	output, err := cmd.Output()
	if err != nil {
		logger.Printf("Ошибка при получении тегов: %v\n", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		fmt.Println("Теги не найдены.")
		logger.Printf("Теги не найдены в репозитории %s", repoURL)
		return
	}

	fmt.Printf("%-30s %-25s %-15s\n", "Тег", "Дата создания", "Тип тега")
	fmt.Println(strings.Repeat("-", 75))

	now := time.Now()
	monthAgo := now.AddDate(0, -1, 0)
	var tagsToDelete []string

	for _, line := range lines {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		tagName := parts[0]
		createdStr := parts[1]
		tagType := getTagType(tmpDir, tagName)

		fmt.Printf("%-30s %-25s %-15s\n", tagName, createdStr, tagType)

		createdTime, err := time.Parse("2006-01-02 15:04:05 -0700", createdStr)
		if err != nil {
			continue
		}

		if !strings.HasPrefix(tagName, "release") && createdTime.Before(monthAgo) {
			tagsToDelete = append(tagsToDelete, tagName)
		}
	}

	fmt.Printf("Всего тегов: %d\n", len(lines))
	logger.Printf("Всего тегов в репозитории %s: %d", repoURL, len(lines))

	// Удаление тегов
	if len(tagsToDelete) > 0 {
		fmt.Println("Удаляются неподходящие теги:")
		logger.Printf("Удаляются теги из репозитория %s:", repoURL)
		for _, tag := range tagsToDelete {
			fmt.Println(" -", tag)
			logger.Printf("Удаление тега: %s", tag)

			// Локальное удаление
			delCmd := exec.Command("git", "tag", "-d", tag)
			delCmd.Dir = tmpDir
			err := delCmd.Run()
			if err != nil {
				logger.Printf("Ошибка при локальном удалении тега %s: %v\n", tag, err)
				continue
			}

			// Удаление на origin
			pushDelCmd := exec.Command("git", "push", "origin", "--delete", tag)
			pushDelCmd.Dir = tmpDir
			err = pushDelCmd.Run()
			if err != nil {
				logger.Printf("Ошибка при удалении тега %s из origin: %v\n", tag, err)
			} else {
				fmt.Printf("Тег %s удалён из origin.\n", tag)
				logger.Printf("Тег %s удалён из origin.", tag)
			}
		}
	} else {
		fmt.Println("Нет тегов для удаления.")
		logger.Printf("Нет тегов для удаления в репозитории %s", repoURL)
	}

	// Проверим оставшиеся теги
	cmd = exec.Command("git", "tag")
	cmd.Dir = tmpDir
	output, err = cmd.Output()
	if err != nil {
		logger.Printf("Ошибка при получении оставшихся тегов: %v\n", err)
		return
	}
	remainingTags := strings.Split(strings.TrimSpace(string(output)), "\n")
	fmt.Printf("Оставшиеся теги: %d\n", len(remainingTags))
	logger.Printf("Оставшиеся теги в репозитории %s: %d", repoURL, len(remainingTags))
}

func getTagType(repoPath, tag string) string {
	cmd := exec.Command("git", "cat-file", "-t", tag)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "неизвестно"
	}
	t := strings.TrimSpace(string(out))
	if t == "tag" {
		return "аннотированный"
	}
	if t == "commit" {
		return "легковесный"
	}
	return t
}
