Assignment 4

To Run The Program:

- The program is in default set to run with 3 clients, this nr. can be changed in the code constant nrOfClients.

- In terminal: go run main.go <id>
  - example: go run main.go 1, go run main.go 2, go run main.go 3

- The data.txt file is used to check if several client have acces to the file at the same time. When a client has acces to the file, they will count from 0 - 9,
so if more client had acces to the file at the same time, this count would be ruined.
