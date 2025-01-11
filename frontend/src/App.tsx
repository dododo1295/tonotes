import Navbar from "./components/NavBar.tsx";
import "./css/App.css";
import Todos from "./pages/Todos.tsx";
import Home from "./pages/Home.tsx";
import { Route, Routes } from "react-router-dom";

function App() {
  return (
    <>
      <Navbar />
      <Routes>
        <Route path="/todos" element={<Todos />}></Route>
      </Routes>
      <Home />
    </>
  );
}

export default App;
