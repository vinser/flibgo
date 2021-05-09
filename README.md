# **flibgo** 
([*русский вариант*](README_RU.md))

**flibgo** is a home library OPDS server 

>The Open Publication Distribution System (OPDS) catalog format is a syndication format for electronic publications based on Atom and HTTP. OPDS catalogs enable the aggregation, distribution, discovery, and acquisition of electronic publications. (Wikipedia)

This **flibgo** release only supports FB2 publications, both individual files and zip archives.

OPDS-catalog is checked and works with mobile readers FBReader and PocketBook Reader with a library of 450,000 publications


## 1-2-3 Installation
---
1. Preparing for installation

   **flibgo** is written in GO and uses MariaDB database to store the catalog, so I recommend launching **flibgo** in Docker containers to simplify installation and setup.

   Desktop installation for Windows, MacOS and Linux is described there https://www.docker.com/products/docker-desktop

2. Setup
   
   Copy zip-archive with **flibgo** `https://github.com/vinser/flibgo/archive/refs/heads/master.zip` or download **flibgo** with `git clone https://github.com/vinser/flibgo.git`

   In the `docker-compose.yml` file specify the folder like 'books' in which FB2 files and/or zip files with FB2s will be processed and stored.

   The folder will contain thre subfolderd:
```
books
  ├─── new   - put new FB2 files and/or zip archives with FB2 files here
  ├─── stock - library catalogue files and archives are stored here
  └─── trash - files that have been processing bugs will come here 
```
   In the `docker-compose.yml` file set your local time zone if needed (TZ tag)

3. Run and Stop

   While in the folder with the docker-compose.yml file run `docker-compose up -d` command to start **flibgo** server.

   **flibgo** will once a minute process new books and add them to the catalog. OPDS catalogue will be available at `http://<your computer's ip>:8085/opds`

   Server shutdown can be done by `docker-compose down` command

## Advanced usage

   For advanced sutup see config/config.yml selfexplanatory file.

   Command `docker-compose exec app go run /flibgo/cmd/flibgo/main.go -reindex` will help to re-create the catalog on the files already processed 

---

*Any comments and suggestions are welcome*
   

