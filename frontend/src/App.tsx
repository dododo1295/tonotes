import Navbar from "./components/NavBar.tsx";
import "./css/App.css";
import Todos from "./pages/Todos.tsx";
import { Routes, Route } from "react-router-dom";

function App() {
  return (
    <>
      <Navbar />
      <Routes>
        <Route path="/todos" element={<Todos />}></Route>
      </Routes>
    </>
  );
}

export default App;
