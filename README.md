# Matchup Lookup

Локальный интерактивный Go CLI для поиска недавних матчей отслеживаемых игроков League of Legends. Показывает Riot ID, соперника по линии, результат и ID матча. История сохраняется в SQLite; видео и `.rofl` не загружаются.

## Установка на Windows

В PowerShell:

```powershell
winget install --id GoLang.Go --exact
winget install --id Casey.Just --exact
winget install --id GitHub.cli --exact
```

Откройте новое окно PowerShell в папке проекта. Проверьте `go version` и `just --version`. Для загрузки списков из приватного GitHub выполните `gh auth login` через браузер. Локальные файлы доступны без GitHub CLI.

## Быстрый старт

```powershell
just
```

В меню выберите **Import players**. По умолчанию программа читает `players/index.json` из приватного GitHub-репозитория. Для локальной копии в поле источника укажите `players` или в PowerShell выполните:

```powershell
just import -source players
```

**Import** сразу записывает Riot ID в SQLite без Riot API и без ключа. Он не скачивает матчи. В `players/fiora/euw/` находятся четыре файла по 50 игроков из присланного списка. Можно добавлять каталоги вроде `players/garen/euw/` или импортировать другой JSON/CSV файл либо HTTP(S) ссылку через `-source`. Формат записи:

```json
{"gameName":"YourPlayer","tagLine":"EUW","region":"euw1","champion":"Fiora"}
```

CSV заголовки: `gameName,tagLine,region,champion,source`; последние два поля необязательны. Один игрок хранится по PUUID и может иметь несколько меток чемпионов. Метка — принадлежность к исходному списку, а поиск фильтрует чемпиона, реально сыгранного в матче. Списки получены из присланной таблицы; Riot API проверит ID во время обновления, и устаревшие записи могут дать ошибку. Редактируйте файлы и повторяйте импорт для добавления игроков.

Затем выберите **Update recent matches**. При первом запуске программа попросит Riot API key с маскированным вводом и сохранит его в локальной SQLite. Меню позволяет заменить истёкший ключ. `RIOT_API_KEY` из окружения имеет приоритет: `$env:RIOT_API_KEY = "RGAPI-your-key"`. Ключ не входит в бинарник и хранится в базе без шифрования; файл базы создаётся с доступом только владельцу. Riot OAuth здесь нет: [RSO требует одобрения Riot](https://developer.riotgames.com/docs/lol).

Обновление проверяет Riot ID, получает PUUID, затем последние 20 рейтинговых Solo/Duo матчей каждого игрока. Детали каждого нового матча Riot Match-V5 отдаёт отдельным запросом. Четыре загрузчика работают параллельно, но общий ограничитель держится ниже лимита 20 запросов/сек и 100 запросов/2 мин: используется максимум 18 и 90 соответственно. На 200 игроков первое полное обновление может занять много времени. Полосы прогресса показывают `N/200` проверенных ID и `N/20` обработанных матчей текущего игрока. При ожидании лимита виден обратный отсчёт и причина паузы. Ctrl+C прерывает обновление; уже записанные данные остаются, повторный запуск не скачивает те же детали матча. HTTP 429 и временные 5xx повторяются с задержкой, `Retry-After` соблюдается.

В **Find matchups** выберите сервер, своего чемпиона и соперника через fuzzy-поиск, затем результат, KDA, ранг и дату. Начните вводить имя, выберите подсказку стрелками и нажмите Enter. Например, `fioa` находит Fiora, `drius` — Darius. **Any champion** снимает соответствующий фильтр. Последние фильтры сохраняются в SQLite и подставляются в форму; **Repeat last search** повторяет их без формы. Каталог чемпионов берётся из Riot Data Dragon и кэшируется в SQLite на семь дней.

Для скриптов:

```powershell
just players
just update
just find -region euw1 -champion Fiora -opponent Darius -result win -kda ge -rank diamond -days 7
just find -champion Garen -days 0
just path
just check
just build
```

`-rank diamond` означает Diamond+, `-days 0` — всю сохранённую историю. `-kda ge` требует KDA игрока не ниже KDA соперника по линии, `-kda gt` — строго выше; победа выбирается отдельно через `-result win`. Пустой `-region` показывает все серверы. Результаты сортируются от новых к старым. `just build` собирает приложение для текущей системы; Windows `.exe` собирается в GitHub Actions.

## Хранение и правила данных

SQLite находится вне репозитория: `%LOCALAPPDATA%\matchup-lookup\matches.db` на Windows, `~/Library/Application Support/matchup-lookup/matches.db` на macOS, `$XDG_DATA_HOME/matchup-lookup/matches.db` или `~/.local/share/matchup-lookup/matches.db` на Linux. Путь можно изменить через `MATCHUP_DB_PATH`. Импортированные ID лежат в `player_seeds`; проверенные PUUID — в `players`; метки чемпионов — в `player_champions`. `matches` хранит детали и исходный JSON матча, `match_checks` — обработанные пары игрок–матч, `tracked_games` — статистику и соперника. Старые матчи не удаляются.

Account-V1 и Match-V5 используют региональный маршрут, League-V4 — платформенный. Например, для EUW используются `europe.api.riotgames.com` и `euw1.api.riotgames.com`. Поддерживаются EUW, EUNE, TR, RU, NA, BR, LAN, LAS, KR, JP и платформы SEA/OCE. Фильтр сервера читает платформу из матча.

Соперник выбирается по одинаковой позиции на другой стороне (`teamPosition` или `individualPosition`). Если они отсутствуют, используется `lane` только при единственном кандидате и строка помечается `lane_fallback`. Неоднозначность помечается `ambiguous`; неизвестный соперник не проходит фильтр его чемпиона и KDA. Ранг — снимок текущего Solo/Duo ранга при обработке матча, а не исторический ранг на дату игры. Доступность повтора в клиенте Riot программа не проверяет.

## Проверка без Riot API

`just check` запускает `go vet` и автономные тесты: локальный импорт 200 ID, проверка накопления меток, маршрутизация, выбор соперника, KDA, кэш, параллельная загрузка и возобновление без повторной загрузки матчей. Riot key и сеть для тестов не нужны.

Маршрутизация и лимиты сверены с [документацией Riot Developer Portal](https://developer.riotgames.com/docs/portal) и [справочником Riot API](https://developer.riotgames.com/apis).
