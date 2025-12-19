#!/bin/bash
# Скрипт для конвертации JPG/PNG изображения в .icns формат
# Использование: ./convert-to-icns.sh <input_image.jpg>

set -e

if [ $# -eq 0 ]; then
    echo "Использование: $0 <input_image.jpg> [output_name.icns]"
    echo ""
    echo "Пример:"
    echo "  $0 app.jpg"
    echo "  $0 app.jpg app.icns"
    exit 1
fi

INPUT_IMAGE="$1"
OUTPUT_NAME="${2:-${INPUT_IMAGE%.*}.icns}"

# Получаем абсолютные пути
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INPUT_PATH="$SCRIPT_DIR/$INPUT_IMAGE"
OUTPUT_PATH="$SCRIPT_DIR/$OUTPUT_NAME"

if [ ! -f "$INPUT_PATH" ]; then
    echo "❌ Ошибка: Файл $INPUT_PATH не найден"
    exit 1
fi

echo ""
echo "========================================"
echo "  Конвертация изображения в .icns"
echo "========================================"
echo ""
echo "Входной файл: $INPUT_IMAGE"
echo "Выходной файл: $OUTPUT_NAME"
echo ""

# Создаём временную папку для .iconset
ICONSET_NAME="${OUTPUT_NAME%.icns}.iconset"
ICONSET_DIR="$SCRIPT_DIR/$ICONSET_NAME"
rm -rf "$ICONSET_DIR"
mkdir -p "$ICONSET_DIR"

echo "=== Создание изображений разных размеров ==="

# Создаём изображения разных размеров для .icns
# macOS требует определённые размеры для разных разрешений экрана
# Важно: явно указываем формат PNG, иначе sips может сохранить исходный формат (JPEG)
sips -s format png -z 16 16     "$INPUT_PATH" --out "$ICONSET_DIR/icon_16x16.png" >/dev/null 2>&1
sips -s format png -z 32 32     "$INPUT_PATH" --out "$ICONSET_DIR/icon_16x16@2x.png" >/dev/null 2>&1
sips -s format png -z 32 32     "$INPUT_PATH" --out "$ICONSET_DIR/icon_32x32.png" >/dev/null 2>&1
sips -s format png -z 64 64     "$INPUT_PATH" --out "$ICONSET_DIR/icon_32x32@2x.png" >/dev/null 2>&1
sips -s format png -z 128 128   "$INPUT_PATH" --out "$ICONSET_DIR/icon_128x128.png" >/dev/null 2>&1
sips -s format png -z 256 256   "$INPUT_PATH" --out "$ICONSET_DIR/icon_128x128@2x.png" >/dev/null 2>&1
sips -s format png -z 256 256   "$INPUT_PATH" --out "$ICONSET_DIR/icon_256x256.png" >/dev/null 2>&1
sips -s format png -z 512 512   "$INPUT_PATH" --out "$ICONSET_DIR/icon_256x256@2x.png" >/dev/null 2>&1
sips -s format png -z 512 512   "$INPUT_PATH" --out "$ICONSET_DIR/icon_512x512.png" >/dev/null 2>&1
sips -s format png -z 1024 1024 "$INPUT_PATH" --out "$ICONSET_DIR/icon_512x512@2x.png" >/dev/null 2>&1

echo "✅ Изображения созданы"

echo ""
echo "=== Конвертация в .icns ==="

# Конвертируем .iconset в .icns
iconutil -c icns "$ICONSET_DIR" -o "$OUTPUT_PATH"

if [ $? -eq 0 ]; then
    echo "✅ Файл создан: $OUTPUT_PATH"
else
    echo "❌ Ошибка при создании .icns файла"
    rm -rf "$ICONSET_DIR"
    exit 1
fi

# Удаляем временную папку
rm -rf "$ICONSET_DIR"

echo ""
echo "========================================"
echo "  Конвертация завершена успешно!"
echo "  Файл: $OUTPUT_NAME"
echo "========================================"
echo ""
