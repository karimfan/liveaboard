import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { ClerkProvider } from "@clerk/clerk-react";

import "./styles/tokens.css";
import "./styles/app.css";

import { Login } from "./pages/Login";
import { Signup } from "./pages/Signup";
import { Dashboard } from "./pages/Dashboard";
import { RequireSession } from "./lib/RequireSession";
import { appConfig } from "./lib/config";
import { clerkAppearance } from "./lib/clerkAppearance";

import { AdminShell } from "./admin/Shell";
import { Overview } from "./admin/pages/Overview";
import { Fleet } from "./admin/pages/Fleet";
import { BoatDetail } from "./admin/pages/BoatDetail";
import { BoatTrips, BoatInventory, BoatNotes } from "./admin/pages/BoatTabs";
import { Trips } from "./admin/pages/Trips";
import { Catalog } from "./admin/pages/Catalog";
import { Users } from "./admin/pages/Users";
import { Organization } from "./admin/pages/Organization";
import { Reports } from "./admin/pages/Reports";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ClerkProvider
      publishableKey={appConfig.clerkPublishableKey}
      appearance={clerkAppearance}
      // Tell Clerk where to send the browser after the hosted flows
      // complete; both routes are within our SPA and handle the next
      // step (org bootstrap or session exchange) themselves.
      signInFallbackRedirectUrl="/"
      signUpFallbackRedirectUrl="/signup"
    >
      <BrowserRouter>
        <Routes>
          <Route path="/signup/*" element={<Signup />} />
          <Route path="/login/*" element={<Login />} />
          <Route
            path="/"
            element={
              <RequireSession>
                <Dashboard />
              </RequireSession>
            }
          />
          {/* Sprint 007 admin UX mockup. Hardcoded data; navigation is real. */}
          <Route
            path="/admin"
            element={
              <RequireSession>
                <AdminShell />
              </RequireSession>
            }
          >
            <Route index element={<Overview />} />
            <Route path="organization" element={<Organization />} />
            <Route path="fleet" element={<Fleet />} />
            <Route path="fleet/:slug" element={<BoatDetail />}>
              <Route index element={<BoatTrips />} />
              <Route path="inventory" element={<BoatInventory />} />
              <Route path="notes" element={<BoatNotes />} />
            </Route>
            <Route path="catalog" element={<Catalog />} />
            <Route path="trips" element={<Trips />} />
            <Route path="users" element={<Users />} />
            <Route path="reports" element={<Reports />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ClerkProvider>
  </React.StrictMode>,
);
