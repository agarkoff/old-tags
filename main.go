package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
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
	defer os.RemoveAll(tmpDir) // Удаляем после завершения

	// Клонируем репозиторий (глубоко, чтобы получить .git)
	fmt.Println("Клонируем репозиторий...")
	cmd := exec.Command("git", "clone", "--quiet", "--mirror", repoURL, tmpDir)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Ошибка при клонировании репозитория: %v", err)
	}

	// Получаем все теги
	fmt.Println("Получаем список тегов...")
	cmd = exec.Command("git", "tag")
	cmd.Dir = tmpDir
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Ошибка при получении тегов: %v", err)
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tags) == 1 && tags[0] == "" {
		fmt.Println("Теги не найдены.")
		fmt.Println("Общее количество тегов: 0")
		return
	}

	// Выводим теги
	fmt.Println("Список тегов:")
	for _, tag := range tags {
		fmt.Println(tag)
	}

	// Выводим количество тегов
	fmt.Printf("Общее количество тегов: %d\n", len(tags))
}
