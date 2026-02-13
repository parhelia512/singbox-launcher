# JSON Schema: Единый config_template.json

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Unified Config Template",
  "description": "Единый шаблон конфигурации sing-box для всех платформ",
  "type": "object",
  "required": ["parser_config", "config", "selectable_rules", "params"],
  "additionalProperties": false,

  "properties": {

    "parser_config": {
      "type": "object",
      "description": "Конфигурация парсера подписок и outbound-групп",
      "required": ["ParserConfig"],
      "additionalProperties": false,
      "properties": {
        "ParserConfig": {
          "type": "object",
          "required": ["version", "proxies", "outbounds"],
          "properties": {
            "version": {
              "type": "integer",
              "description": "Версия формата ParserConfig"
            },
            "proxies": {
              "type": "array",
              "description": "Источники прокси (подписки, прямые ссылки)",
              "items": {
                "type": "object",
                "required": ["source"],
                "properties": {
                  "source": {
                    "type": "string",
                    "description": "URL подписки или прямая ссылка на прокси"
                  }
                }
              }
            },
            "outbounds": {
              "type": "array",
              "description": "Outbound-группы, генерируемые парсером",
              "items": { "$ref": "#/definitions/parser_outbound" }
            },
            "parser": {
              "type": "object",
              "description": "Настройки парсера (интервал обновления, время последнего обновления)",
              "properties": {
                "reload": {
                  "type": "string",
                  "description": "Интервал автообновления (Go duration: 1h, 30m, etc.)"
                },
                "last_updated": {
                  "type": "string",
                  "format": "date-time",
                  "description": "Время последнего обновления (RFC3339)"
                }
              }
            }
          }
        }
      }
    },

    "config": {
      "type": "object",
      "description": "Основной конфиг sing-box (log, dns, inbounds, outbounds, route, experimental). Платформонезависимая часть.",
      "properties": {
        "log": { "type": "object" },
        "dns": { "type": "object" },
        "inbounds": {
          "type": "array",
          "description": "Пустой массив — заполняется из params по платформе",
          "items": { "type": "object" }
        },
        "outbounds": {
          "type": "array",
          "description": "Статические outbound-ы (direct-out). Сгенерированные группы вставляются парсером.",
          "items": { "type": "object" }
        },
        "route": {
          "type": "object",
          "properties": {
            "rule_set": {
              "type": "array",
              "description": "Только общие rule_set, используемые несколькими правилами или DNS. Остальные привязаны к selectable_rules.",
              "items": { "$ref": "#/definitions/rule_set_definition" }
            },
            "rules": {
              "type": "array",
              "description": "Только базовые универсальные правила (hijack-dns, ip_is_private, local). Selectable правила — в отдельной секции.",
              "items": { "type": "object" }
            },
            "final": { "type": "string" },
            "auto_detect_interface": { "type": "boolean" }
          }
        },
        "experimental": { "type": "object" }
      }
    },

    "selectable_rules": {
      "type": "array",
      "description": "Правила маршрутизации для визарда. Пользователь включает/выключает их в UI.",
      "items": { "$ref": "#/definitions/selectable_rule" }
    },

    "params": {
      "type": "array",
      "description": "Платформозависимые параметры. Применяются к config при загрузке шаблона.",
      "items": { "$ref": "#/definitions/param" }
    }
  },

  "definitions": {

    "parser_outbound": {
      "type": "object",
      "description": "Outbound-группа, генерируемая парсером из подписок",
      "required": ["tag", "type"],
      "properties": {
        "tag": {
          "type": "string",
          "description": "Уникальный тег outbound-группы"
        },
        "type": {
          "type": "string",
          "enum": ["urltest", "selector"],
          "description": "Тип группы: urltest (автовыбор) или selector (ручной выбор)"
        },
        "wizard": {
          "type": "object",
          "description": "Метаданные для визарда",
          "properties": {
            "required": {
              "type": "integer",
              "description": "Уровень обязательности: 1 = обязательный, 2 = служебный"
            },
            "hide": {
              "type": "boolean",
              "description": "Скрыть из UI визарда"
            }
          }
        },
        "options": {
          "type": "object",
          "description": "Опции sing-box для outbound-группы"
        },
        "filters": {
          "type": "object",
          "description": "Фильтры для отбора прокси из подписок"
        },
        "addOutbounds": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Дополнительные outbound-ы для добавления в группу"
        },
        "comment": {
          "type": "string",
          "description": "Комментарий для отображения в UI"
        }
      }
    },

    "rule_set_definition": {
      "type": "object",
      "description": "Определение rule_set для sing-box",
      "required": ["tag", "type", "format"],
      "properties": {
        "tag": {
          "type": "string",
          "description": "Уникальный тег rule_set"
        },
        "type": {
          "type": "string",
          "enum": ["inline", "remote"],
          "description": "inline — правила встроены, remote — загружаются по URL"
        },
        "format": {
          "type": "string",
          "enum": ["domain_suffix", "binary"],
          "description": "Формат данных rule_set"
        },
        "url": {
          "type": "string",
          "description": "URL для загрузки (только для remote)"
        },
        "download_detour": {
          "type": "string",
          "description": "Outbound для загрузки (только для remote)"
        },
        "update_interval": {
          "type": "string",
          "description": "Интервал обновления (только для remote)"
        },
        "rules": {
          "type": "array",
          "description": "Встроенные правила (только для inline)",
          "items": { "type": "object" }
        }
      }
    },

    "selectable_rule": {
      "type": "object",
      "description": "Правило маршрутизации, управляемое пользователем в визарде",
      "required": ["label", "description"],
      "properties": {
        "label": {
          "type": "string",
          "description": "Название правила для отображения в UI"
        },
        "description": {
          "type": "string",
          "description": "Описание правила (tooltip в визарде)"
        },
        "default": {
          "type": "boolean",
          "default": false,
          "description": "Включено по умолчанию при первом запуске визарда"
        },
        "platforms": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": ["windows", "linux", "darwin"]
          },
          "description": "Платформы, на которых правило доступно. Если не указано — доступно на всех платформах."
        },
        "rule_set": {
          "type": "array",
          "description": "Определения rule_set, необходимые этому правилу. Добавляются в config.route.rule_set только если правило включено.",
          "items": { "$ref": "#/definitions/rule_set_definition" }
        },
        "rule": {
          "type": "object",
          "description": "Одиночное правило маршрутизации sing-box"
        },
        "rules": {
          "type": "array",
          "description": "Несколько правил маршрутизации (взаимоисключающее с rule)",
          "items": { "type": "object" }
        }
      },
      "oneOf": [
        { "required": ["rule"] },
        { "required": ["rules"] }
      ]
    },

    "param": {
      "type": "object",
      "description": "Платформозависимый параметр, применяемый к config",
      "required": ["name", "platforms", "value"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Путь к секции в config (точечная нотация: inbounds, route.rules)"
        },
        "platforms": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": ["win", "linux", "darwin"]
          },
          "description": "Платформы, на которых применяется параметр"
        },
        "mode": {
          "type": "string",
          "enum": ["replace", "prepend", "append"],
          "default": "replace",
          "description": "Режим применения: replace — заменить, prepend — вставить в начало массива, append — добавить в конец массива"
        },
        "value": {
          "description": "Значение для подстановки в config[name]"
        }
      }
    }

  }
}
```

