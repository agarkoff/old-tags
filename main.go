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

	// Переменные для подсчета общей статистики
	totalRemainingTags := 0
	totalDeletedTags := 0

	// Обработка каждого репозитория
	for i, repoURL := range repos {
		fmt.Printf("\n🔧 Обработка репозитория %d/%d: %s\n", i+1, len(repos), repoURL)
		logger.Printf("Обработка репозитория %d/%d: %s", i+1, len(repos), repoURL)

		remainingInRepo, deletedInRepo := processRepository(repoURL)
		totalRemainingTags += remainingInRepo
		totalDeletedTags += deletedInRepo

		fmt.Printf("\n📊 Итого оставшихся тегов в репозитории: %d\n", remainingInRepo)
		fmt.Println(strings.Repeat("=", 80))
	}

	// Вывод общей статистики
	fmt.Printf("\n🎯 ОБЩАЯ СТАТИСТИКА:\n")
	fmt.Printf("Обработано репозиториев: %d\n", len(repos))
	fmt.Printf("Общее количество удаленных тегов: %d\n", totalDeletedTags)
	fmt.Printf("Общее количество оставшихся тегов во всех репозиториях: %d\n", totalRemainingTags)

	logger.Printf("Завершена обработка %d репозиториев. Удалено тегов: %d, осталось тегов: %d", len(repos), totalDeletedTags, totalRemainingTags)
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

func processRepository(repoURL string) (int, int) {
	tmpDir, err := os.MkdirTemp("", "git-tags-*")
	if err != nil {
		logger.Printf("Не удалось создать временную директорию: %v\n", err)
		return 0, 0
	}
	defer os.RemoveAll(tmpDir)

	// Клонируем репозиторий
	fmt.Println("Клонируем репозиторий...")
	logger.Printf("Клонирование репозитория %s", repoURL)
	cmd := exec.Command("git", "clone", "--quiet", repoURL, tmpDir)
	if err := cmd.Run(); err != nil {
		logger.Printf("Ошибка при клонировании репозитория %s: %v\n", repoURL, err)
		return 0, 0
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
		return 0, 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		fmt.Println("Теги не найдены.")
		logger.Printf("Теги не найдены в репозитории %s", repoURL)
		return 0, 0
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

	fmt.Printf("\nВсего тегов до обработки: %d\n", len(lines))
	logger.Printf("Всего тегов в репозитории %s: %d", repoURL, len(lines))

	// Удаление тегов
	deletedCount := 0
	if len(tagsToDelete) > 0 {
		fmt.Printf("Удаляются неподходящие теги (%d штук):\n", len(tagsToDelete))
		logger.Printf("Удаляются %d тегов из репозитория %s:", len(tagsToDelete), repoURL)

		for _, tag := range tagsToDelete {
			fmt.Printf(" - Удаление: %s", tag)
			logger.Printf("Удаление тега: %s", tag)

			// Локальное удаление
			delCmd := exec.Command("git", "tag", "-d", tag)
			delCmd.Dir = tmpDir
			err := delCmd.Run()
			if err != nil {
				fmt.Printf(" [ОШИБКА локального удаления]\n")
				logger.Printf("Ошибка при локальном удалении тега %s: %v\n", tag, err)
				continue
			}

			// Удаление на origin
			pushDelCmd := exec.Command("git", "push", "origin", "--delete", tag)
			pushDelCmd.Dir = tmpDir
			err = pushDelCmd.Run()
			if err != nil {
				fmt.Printf(" [ОШИБКА удаления из origin]\n")
				logger.Printf("Ошибка при удалении тега %s из origin: %v\n", tag, err)
			} else {
				fmt.Printf(" [OK]\n")
				logger.Printf("Тег %s успешно удалён из origin.", tag)
				deletedCount++
			}
		}
		fmt.Printf("\nУспешно удалено тегов: %d из %d\n", deletedCount, len(tagsToDelete))
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
		return 0, deletedCount
	}

	remainingTags := []string{}
	if strings.TrimSpace(string(output)) != "" {
		remainingTags = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	remainingCount := len(remainingTags)
	if remainingCount == 1 && remainingTags[0] == "" {
		remainingCount = 0
	}

	fmt.Printf("\n📈 Статистика по репозиторию:\n")
	fmt.Printf("   Было тегов: %d\n", len(lines))
	fmt.Printf("   Удалено тегов: %d\n", deletedCount)
	fmt.Printf("   Осталось тегов: %d\n", remainingCount)

	logger.Printf("Статистика для репозитория %s: было %d, удалено %d, осталось %d", repoURL, len(lines), deletedCount, remainingCount)

	return remainingCount, deletedCount
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
