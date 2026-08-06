import { useLocation } from "react-router-dom";
import MaituoConsole from "./MaituoConsole";
import SubaccountFiles from "./SubaccountFiles";

function App() {
  const location = useLocation();
  if (location.pathname.startsWith("/subaccount-files/")) return <SubaccountFiles />;
  return <MaituoConsole />;
}

export default App;
