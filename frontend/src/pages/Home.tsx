import MainContext from "../contexts/MainContext.tsx";
import TodoContext from "../contexts/TodoContext.tsx";

function Home(): JSX.Element {
  return (
    <>
      <div>
        <h1>Welcome to toNotes</h1>
      </div>
      <MainContext />
      <TodoContext />
    </>
  );
}

export default Home;
