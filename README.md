# Fiora Matchup Lookup

Небольшой локальный Go CLI для поиска недавних матчей Fiora из своего списка игроков EUW. Хранит историю в SQLite, показывает Riot ID и ID матча, чтобы найти запись в клиенте League. Видео и `.rofl` не скачивает.

## Установка на Windows

В PowerShell:

```powershell
winget install --id GoLang.Go --exact
winget install --id Casey.Just --exact
```

Откройте новое окно PowerShell в каталоге проекта и проверьте `go version` и `just --version`. Go-модули загрузятся при первом запуске. Пакеты: чистый Go SQLite-драйвер `modernc.org/sqlite` и оформление CLI `lipgloss`.

## Быстрый старт

1. Заполните `players.json` в приватном GitHub-репозитории `MrWaip/matchup-lookup` реальными Riot ID. Формат:

   ```json
   [
     {"gameName":"YourFioraPlayer","tagLine":"EUW","region":"euw1"}
   ]
   ```

   По умолчанию `import` загружает этот GitHub-файл. Можно указать локальный файл или другую HTTP(S) ссылку: `just import -source players.json` либо `just import -source https://example.com/players.csv`. Заголовки CSV: `gameName,tagLine,region,source`. Поле `source` необязательно и позволяет позже подключить внешний источник списка без изменения сборщика. В MVP поддерживается EUW (`euw1`). Для приватного GitHub-файла нужен `GITHUB_TOKEN` с правом чтения содержимого репозитория или выполненный `gh auth login` на этой машине.

2. Установите ключ Riot API только в текущем сеансе PowerShell:

   ```powershell
   $env:RIOT_API_KEY = "RGAPI-your-key"
   just import
   just players
   just update
   ```

   Истёкший development key просто замените тем же способом. Ключ не сохраняется в базе и не передаётся в URL. Если хотите импортировать локальный файл без GitHub-доступа, выполните `just import -source players.json`.

3. Ищите игры без ключа и без сетевых запросов:

   ```powershell
   just find -opponent Darius -result win -rank diamond -days 7
   ```

   `-rank diamond` означает Diamond+, `-days 0` — вся сохранённая история. Результаты идут от новых к старым. Riot ID Fiora показан отдельной колонкой, его можно выделить и скопировать из терминала.

4. Полезные команды:

   ```powershell
   just find -opponent Darius -result any -rank emerald -days 14 -min-minutes 15
   just find -patch 16.19 -player YourFioraPlayer -days 0
   just path
   just check
   just build
   ```

   `just build` собирает бинарник для текущей системы. Windows `.exe` собирается в GitHub Actions и прикладывается к запуску workflow как artifact. Команда `just find` без аргументов показывает последние 7 дней.

## Где база

SQLite хранится вне репозитория: `%LOCALAPPDATA%\matchup-lookup\matches.db` на Windows, `~/Library/Application Support/matchup-lookup/matches.db` на macOS, `$XDG_DATA_HOME/matchup-lookup/matches.db` или `~/.local/share/matchup-lookup/matches.db` на Linux. Это постоянные данные приложения; чистка обычного временного кэша их не удаляет. Для другого пути задайте `MATCHUP_DB_PATH`.

Игроки после импорта хранятся в таблице `players`; `update` читает список только из неё. Матчи не удаляются после выхода из последних 20 игр. `matches` хранит даже матчи без Fiora и нужные поля ответа Riot; `match_checks` отмечает обработку каждого игрока; `fiora_games` хранит найденные игры. Поэтому общий матч двух игроков скачивается один раз, а повторный `update` пропускает уже обработанные пары игрок–матч. Если один игрок сначала сыграл не Fiora, его матч тоже не скачивается повторно.

## Правила данных

- Account-V1 (`by-riot-id`) и Match-V5 (`ids`, детали матча) используют региональный хост `europe.api.riotgames.com`. League-V4 (`entries/by-puuid`) использует платформенный `euw1.api.riotgames.com`.
- Берутся последние 20 матчей очереди 420 (Ranked Solo/Duo) каждого игрока. При обнаружении Fiora берётся противник из противоположной команды с позицией `TOP` по `teamPosition` или `individualPosition`. Если оба поля пусты, возможен запасной вариант `lane=TOP`; строка помечается `lane_fallback`. Конфликт или несколько кандидатов помечаются `ambiguous`, а соперник остаётся неизвестным. Если позиция Fiora не подтверждена как TOP, статус `fiora_not_confirmed_top`.
- Ранг — снимок текущего Solo/Duo ранга **на момент обработки матча**, а не исторический ранг на дату игры. Нерангированные и матчи, для которых запрос ранга не удался, не проходят фильтр Diamond+.
- Клиент делает запросы последовательно с паузой 1.2 секунды и повторяет 429/временные 5xx с задержкой; `Retry-After` имеет приоритет. Ошибки отдельных игроков/матчей показываются в консоли; уже собранные данные сохраняются.
- Доступность повтора в клиенте Riot этим инструментом не проверяется.

## Проверка без Riot API

`just check` запускает `go vet` и автономные Go-тесты с подставным клиентом и HTTP-сервером: они проверяют импорт файла/URL, выбор соперника, фильтры и отсутствие повторной загрузки матча. Реальный Riot API и ключ для тестов не нужны.

Маршрутизация и работа с идентификаторами сверены с [документацией Riot по LoL API](https://developer.riotgames.com/docs/lol) и [справочником API](https://developer.riotgames.com/apis). Поведение `Retry-After` описано в [документации Riot Developer Portal](https://developer.riotgames.com/docs/portal).
