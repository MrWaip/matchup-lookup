# Matchup Lookup

Небольшой локальный интерактивный Go CLI для поиска недавних матчей **любого чемпиона** из своего списка игроков EUW. После запуска открывается меню: импорт игроков, обновление матчей, поиск и просмотр списка. Хранит историю в SQLite, показывает Riot ID и ID матча, чтобы найти запись в клиенте League. Видео и `.rofl` не скачивает.

## Установка на Windows

В PowerShell:

```powershell
winget install --id GoLang.Go --exact
winget install --id Casey.Just --exact
```

Откройте новое окно PowerShell в каталоге проекта и проверьте `go version` и `just --version`. Go-модули загрузятся при первом запуске. Пакеты: чистый Go SQLite-драйвер `modernc.org/sqlite` и терминальные формы Charm `huh`/`lipgloss`.

## Быстрый старт

1. Заполните `players.json` в приватном GitHub-репозитории `MrWaip/matchup-lookup` реальными Riot ID. Формат:

   ```json
   [
     {"gameName":"YourPlayer","tagLine":"EUW","region":"euw1"}
   ]
   ```

   По умолчанию `import` загружает этот GitHub-файл. Можно указать локальный файл или другую HTTP(S) ссылку: `just import -source players.json` либо `just import -source https://example.com/players.csv`. Заголовки CSV: `gameName,tagLine,region,source`. Поле `source` необязательно и позволяет позже подключить внешний источник списка без изменения сборщика. В MVP поддерживается EUW (`euw1`). Для приватного GitHub-файла нужен `GITHUB_TOKEN` с правом чтения содержимого репозитория или выполненный `gh auth login` на этой машине.

2. Запустите приложение:

   ```powershell
   just
   ```

   В меню сначала выберите **Import players**, затем **Update recent matches**, потом **Find matchups**. Для Riot API приложение спросит ключ с маскированным вводом, если `RIOT_API_KEY` не установлен. Ключ не сохраняется в базе. При необходимости можно задать его в текущем сеансе PowerShell: `$env:RIOT_API_KEY = "RGAPI-your-key"`. Истёкший development key просто замените. Riot OAuth-логина в этой версии нет. Для локального списка в меню укажите `players.json`.

3. Поиск в меню работает без ключа и без сетевых запросов. Введите своего чемпиона и соперника, выберите результат игры, сравнение KDA, ранг и срок. Дополнительные фильтры доступны на следующем шаге. Например, победа Fiora над Darius с KDA не хуже соперника:

   ```powershell
   just find -champion Fiora -opponent Darius -result win -kda ge -rank diamond -days 7
   ```

   `-rank diamond` означает Diamond+, `-days 0` — вся сохранённая история. `-kda ge` сравнивает коэффициент `(убийства + ассисты) / max(1, смерти)` у игрока и соперника по линии; `-kda gt` требует строго большего значения. Результат игры — отдельный фильтр, поэтому их можно сочетать. Если соперник не определён, сравнение KDA не проходит. Результаты идут от новых к старым. Riot ID игрока показан отдельной колонкой, его можно скопировать из терминала.

4. Полезные команды для автоматизации:

   ```powershell
   just import -source players.json
   just players
   just update
   just find -champion Garen -opponent Darius -result any -rank emerald -days 14 -min-minutes 15
   just find -patch 16.19 -player YourPlayer -days 0
   just path
   just check
   just build
   ```

   `just build` собирает бинарник для текущей системы. Windows `.exe` собирается в GitHub Actions и прикладывается к запуску workflow как artifact. `just run` также открывает интерактивное меню. Команда `just find` без аргументов показывает последние 7 дней.

## Где база

SQLite хранится вне репозитория: `%LOCALAPPDATA%\matchup-lookup\matches.db` на Windows, `~/Library/Application Support/matchup-lookup/matches.db` на macOS, `$XDG_DATA_HOME/matchup-lookup/matches.db` или `~/.local/share/matchup-lookup/matches.db` на Linux. Это постоянные данные приложения; чистка обычного временного кэша их не удаляет. Для другого пути задайте `MATCHUP_DB_PATH`.

Игроки после импорта хранятся в таблице `players`; `update` читает список только из неё. Матчи не удаляются после выхода из последних 20 игр. `matches` хранит полученные детали матча; `match_checks` отмечает обработку каждого игрока; `tracked_games` хранит сыгранные отслеживаемыми игроками матчи любого чемпиона. Поэтому общий матч двух игроков скачивается один раз, а повторный `update` пропускает уже обработанные пары игрок–матч. При переходе со старой схемы Fiora закэшированные матчи переносятся без повторного запроса Riot.

## Правила данных

- Account-V1 (`by-riot-id`) и Match-V5 (`ids`, детали матча) используют региональный хост `europe.api.riotgames.com`. League-V4 (`entries/by-puuid`) использует платформенный `euw1.api.riotgames.com`.
- Берутся последние 20 матчей очереди 420 (Ranked Solo/Duo) каждого игрока. Соперник определяется по совпадающей роли в противоположной команде (`TOP`, `JUNGLE`, `MIDDLE`, `BOTTOM`, `UTILITY`) из `teamPosition` или `individualPosition`. Если оба поля пусты, используется `lane` там, где он позволяет выбрать одного кандидата; такая строка помечается `lane_fallback`. Конфликт или несколько кандидатов помечаются `ambiguous`, а соперник остаётся неизвестным. Для неизвестной роли игрока статус `player_position_unknown`.
- Ранг — снимок текущего Solo/Duo ранга **на момент обработки матча**, а не исторический ранг на дату игры. Нерангированные и матчи, для которых запрос ранга не удался, не проходят фильтр Diamond+.
- Клиент делает запросы последовательно с паузой 1.2 секунды и повторяет 429/временные 5xx с задержкой; `Retry-After` имеет приоритет. Ошибки отдельных игроков/матчей показываются в консоли; уже собранные данные сохраняются.
- Доступность повтора в клиенте Riot этим инструментом не проверяется.

## Проверка без Riot API

`just check` запускает `go vet` и автономные Go-тесты с подставным клиентом и HTTP transport: они проверяют импорт файла/URL, выбор соперника, фильтры KDA и отсутствие повторной загрузки матча. Реальный Riot API и ключ для тестов не нужны.

Маршрутизация и работа с идентификаторами сверены с [документацией Riot по LoL API](https://developer.riotgames.com/docs/lol) и [справочником API](https://developer.riotgames.com/apis). Поведение `Retry-After` описано в [документации Riot Developer Portal](https://developer.riotgames.com/docs/portal).
