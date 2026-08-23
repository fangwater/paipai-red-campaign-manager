import { useLocation } from "react-router-dom";
import MaituoConsole from "./MaituoConsole";
import SubaccountFiles from "./SubaccountFiles";
import ProviderFiles from "./ProviderFiles";

function App() {
  const location = useLocation();
  if (location.pathname.startsWith("/subaccount-files/")) return <SubaccountFiles />;
  if (location.pathname.startsWith("/provider-files/")) return <ProviderFiles />;
  return <MaituoConsole />;
}

export default App;
