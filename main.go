package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Укажите адрес Git-репозитория как аргумент")
	}
	repoURL := os.Args[1]

	// Создаем временную директорию
	tmpDir, err := os.MkdirTemp("", "git-tags-*")
	if err != nil {
		log.Fatalf("Не удалось создать временную директорию: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Клонируем репозиторий
	fmt.Println("Клонируем репозиторий...")
	cmd := exec.Command("git", "clone", "--quiet", repoURL, tmpDir)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Ошибка при клонировании репозитория: %v", err)
	}

	// Получаем список тегов с датами
	fmt.Println("Получаем список тегов и дат...")
	cmd = exec.Command("git", "for-each-ref",
		"--sort=creatordate",
		"--format=%(refname:short)|%(creatordate:iso8601)",
		"refs/tags")
	cmd.Dir = tmpDir
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Ошибка при получении тегов: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		fmt.Println("Теги не найдены.")
		fmt.Println("Общее количество тегов: 0")
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

	fmt.Printf("\nОбщее количество тегов: %d\n", len(lines))

	// Удаление ненужных тегов
	if len(tagsToDelete) > 0 {
		fmt.Printf("\nУдаляются теги, не начинающиеся с 'release' и старше 1 месяца:\n")
		for _, tag := range tagsToDelete {
			fmt.Println("Удаление тега:", tag)

			// Удаление локального тега
			delCmd := exec.Command("git", "tag", "-d", tag)
			delCmd.Dir = tmpDir
			err := delCmd.Run()
			if err != nil {
				fmt.Printf("Ошибка при локальном удалении тега %s: %v\n", tag, err)
				continue
			}

			// Удаление из удалённого репозитория
			pushDelCmd := exec.Command("git", "push", "origin", "--delete", tag)
			pushDelCmd.Dir = tmpDir
			err = pushDelCmd.Run()
			if err != nil {
				fmt.Printf("Ошибка при удалении тега %s из origin: %v\n", tag, err)
			} else {
				fmt.Printf("Тег %s удалён из origin.\n", tag)
			}
		}
	} else {
		fmt.Println("\nНет тегов для удаления.")
	}
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
