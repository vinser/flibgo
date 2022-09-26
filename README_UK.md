# **flibgo**

### СТАБІЛЬНИЙ РЕЛІЗ v1.1.1

*Мультиплатформенний сервіс розробляється у вигляді полегшеного виконуваного модуля* [***flibgolite***](https://github.com/vinser/flibgolite)

*За результатами розробки та тестування ***flibgolite*** буде зроблено оновлення ***flibgo***

---

**flibgo** это OPDS-сервер для домашней библиотеки

>The Open Publication Distribution System (OPDS) catalog format is a syndication format for electronic publications based on Atom and HTTP. OPDS catalogs enable the aggregation, distribution, discovery, and acquisition of electronic publications. (Wikipedia)


Цей випуск **flibgo** підтримує лише публікації FB2, як окремі файли, так і архіви zip.

OPDS-каталог перевірений і працює з мобільними зчитувачами FBReader і PocketBook Reader


## Встановлення як 1-2-3
---
1. Підготовка до встановлення

**flibgo** написаний мовою GO і для зберігання каталогу використовує СУБД MySQL, тому для спрощення встановлення та налаштування рекомендую запускати **flibgo** у контейнерах Docker.

   Порядок встановлення Docker Desktop для Windows, MacOS та Linux описаний [тут](https://www.docker.com/products/docker-desktop)

2. Налаштування
   
   Скопіюйте zip-архів за допомогою **flibgo** `https://github.com/vinser/flibgo/archive/refs/heads/master.zip` або завантажте **flibgo** за допомогою `git clone https://github.com/vinser/flibgo.git`
   
У файлі `docker-compose.yml` вкажіть папку, наприклад 'books', у якій будуть оброблятися та зберігатися файли FB2 та/або zip-файли з FB2.
   
   Папка міститиме дві вкладені папки:
```
books
  ├─── stock - сюди розмістіть нові файли FB2 та/або zip-архіви з файлами FB2
  └─── trash - сюди потраплять файли, які обробляли помилки
```

3. Запуск та зупинення

   Перебуваючи в папці з файлом docker-compose.yml, запустіть **flibgo** командою `docker-compose up -d`
   
   **flibgo** буде раз на хвилину каталогізувати нові книги та надасть доступ до OPDS-каталогу за URL `http://<ip або ім'я вашого комп'ютера>:8085/opds`

   Зупинення сервера здійснюється командою `docker-compose down`

## Розширене використання
---

   Перездати каталог за вже обробленим файлом допоможе команда `docker-compose exec app go run /flibgo/cmd/flibgo/main.go -reindex`

   Додаткові налаштування дивитись в `confif/config.yml` Там все очевидно ;)

---
*Критика та пропозиції вітаються, але прошу не нарікати, сервер пишеться у вільний час*
   


