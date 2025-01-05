import "../css/NavBar.css";
import { Link } from "react-router-dom";

function NavBar() {
  return (
    <nav className="navbar">
      <div className="navbar-brand">
        <Link to="/">toNotes</Link>
      </div>
      <div className="navbar-links">
        <Link to="/todos" className="navbar-link">
          Todos
        </Link>
        <Link to="/notes" className="navbar-link">
          Notes
        </Link>
        <Link to="/account" className="navbar-link">
          Account
        </Link>
      </div>
    </nav>
  );
}
export default NavBar;
