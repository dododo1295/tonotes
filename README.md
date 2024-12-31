# toNotes (WIP)
## Safe, Secure Todo and Notes Webapp
### **Severely Alpha**

The goal of this project is to create a todo and note app webapp. When complete this app will have full encryption and double authentification for notes. This project has no timeline but will be what I work on for the forseeable future outside projects to learn how to make this project. This is my first real project so the scale may decrease as I move for the goal to hit MVP and a level of usefulness by May 2025. (I'm confident that it will hit completion before then though)

Basically I want to make a safe, private, lightweight app that will replace a todo app and a capture app.

For my brain apps like Evernote, Notion, and Obsidian get in the way and are far to complicated to get set up and going quickly. the slight trade off in customizability for a more direct user experience is more important for me, and hopefully for others. The focus of this app is for capturing important things and being able to check todos.

___

## The Stack

- React front-end in TypeScript (will learn this as we go...)
- Tailwind for CSS
- Go Server for account management AND encrypt and decrypt in and out of Databases
- MongoDB database

*currently debating the use of python for the back-end as the database will be written in python anyway.

- [Color Scheme Inspo #1](https://www.color-hex.com/color-palette/1016895)

- [Color Scheme Inspo #2](https://www.color-hex.com/color-palette/60652)

- [Color Scheme Inspo #3](https://www.color-hex.com/color-palette/3307)



---





## Todo
### Up next

[] learn more Go...

[] (Account Creation) figure out how to add and remove from my database via the server or react...


### General Todos (constantly changing as I learn more)

[] create a Go server that can be in the middle for MongoDB and React and test that this works without encryption

[] look into HashiCorp Vault or AWS Secrets Manager

[] create seperate requirements for logins and encryption keys but it's paramount that only the login can access their own todos/notes.

[] create HTML template for front-end

[] figure out the typescript

[] structure the components.

[] figure out UX.

[] how to encrypt database? - encrypt before put in and decrypt on the way out, maybe?

[] finalize name

[] figure out how to store notes and how they should be formatted (can I use md files? how would I encrypt/store them)

### Completed

[x] create MongoDB database.

[x] import the necessary libraries for tailwind
