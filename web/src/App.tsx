import { Routes, Route, NavLink } from "react-router-dom";
import Dashboard from "@/components/Dashboard";
import PersonaManager from "@/components/PersonaManager";

function NavItem({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        `px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
          isActive
            ? "bg-gray-800 text-white"
            : "text-gray-400 hover:text-gray-200 hover:bg-gray-800/50"
        }`
      }
    >
      {children}
    </NavLink>
  );
}

export default function App() {
  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      <header className="border-b border-gray-800 bg-gray-950/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-14">
            <div className="flex items-center gap-6">
              <h1 className="text-lg font-semibold tracking-tight text-white">
                Nimbus
              </h1>
              <nav className="flex items-center gap-1">
                <NavItem to="/">Dashboard</NavItem>
                <NavItem to="/personas">Personas</NavItem>
              </nav>
            </div>
          </div>
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/personas" element={<PersonaManager />} />
        </Routes>
      </main>
    </div>
  );
}
