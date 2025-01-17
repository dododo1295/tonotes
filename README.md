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



## Backend Features Implemented

### Authentication & Authorization
- ✅ User Registration with validation
- ✅ Login with JWT token generation
- ✅ Refresh token mechanism
- ✅ Token blacklisting
- ✅ Session management
- ✅ Logout (single session)
- ✅ Logout all sessions

### User Management
- ✅ Get user profile
- ✅ Change email (with rate limiting)
- ✅ Change password (with rate limiting)
- ✅ Delete account
- ✅ Password validation and hashing (Argon2)

### Session Management
- ✅ Session tracking
- ✅ Multiple device sessions
- ✅ Session expiration
- ✅ Active session listing
- ✅ Session termination

### Statistics
- ✅ User statistics
  - Notes count (total, archived, pinned)
  - Todo count (total, completed, pending)
  - Activity tracking
  - Tag statistics
  - Session count

### Notes (Repository Ready)
- ✅ CRUD operations
- ✅ Archiving functionality
- ✅ Pin/Unpin notes
- ✅ Tag system
- ✅ Search functionality
- ✅ Tag-based filtering

### Todos (Repository Ready)
- ✅ CRUD operations
- ✅ Complete/Incomplete toggle
- ✅ Status filtering (completed/pending)

### Security Features
- ✅ JWT Authentication
- ✅ Password hashing with Argon2
- ✅ Rate limiting for sensitive operations
- ✅ Input validation
- ✅ CORS protection

### Testing
- ✅ Authentication tests
- ✅ User management tests
- ✅ Session management tests
- ✅ Statistics tests
- ✅ Database connection tests

### Infrastructure
- ✅ MongoDB integration
- ✅ Environment configuration
- ✅ Error handling
- ✅ Response standardization
- ✅ Middleware implementation

### Currently Implementing
1. Notes Handler Implementation
2. Todos Handler Implementation


---

## License
This project is licensed under an **All Rights Reserved** license. Unauthorized copying, distribution, or use of this material is strictly prohibited.

---
