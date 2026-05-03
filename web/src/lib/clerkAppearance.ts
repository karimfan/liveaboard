// Clerk's Appearance type is large and we only set a slice of it; using a
// loose type here keeps the file independent of @clerk/types (which is a
// transitive dep of @clerk/clerk-react and not always exported directly).
type Appearance = Record<string, unknown>;

// Map DESIGN.md tokens onto Clerk's appearance variables. Mirrors
// web/src/styles/tokens.css; keep them in sync if either changes.
//
// Clerk's variables are themeable at the ClerkProvider level. The few
// `elements` overrides below smooth over chrome that doesn't match the
// industrial/utilitarian aesthetic — most notably the rounded primary
// button and the headline font.
export const clerkAppearance: Appearance = {
  variables: {
    colorPrimary: "#e5853b",
    colorBackground: "#ffffff",
    colorText: "#1c1917",
    colorTextSecondary: "#57534e",
    colorTextOnPrimaryBackground: "#ffffff",
    colorInputBackground: "#ffffff",
    colorInputText: "#1c1917",
    colorNeutral: "#44403c",
    colorDanger: "#dc3545",
    colorSuccess: "#2d9d5c",
    colorWarning: "#d4930c",
    fontFamily: '"DM Sans", system-ui, sans-serif',
    fontFamilyButtons: '"DM Sans", system-ui, sans-serif',
    fontSize: "1rem",
    borderRadius: "8px",
    spacingUnit: "1rem",
  },
  elements: {
    // Drop Clerk's outer card chrome — our page already provides one.
    rootBox: { width: "100%" },
    card: {
      boxShadow: "none",
      border: "none",
      padding: 0,
      backgroundColor: "transparent",
    },
    cardBox: {
      boxShadow: "none",
      border: "none",
      backgroundColor: "transparent",
    },
    // Our auth pages render a "Liveaboard" wordmark above the Clerk
    // component; suppressing Clerk's own header keeps a single visual
    // hierarchy and avoids "Liveaboard / Sign in to Liveaboard" stacking.
    header: { display: "none" },
    headerTitle: { display: "none" },
    headerSubtitle: { display: "none" },
    formButtonPrimary: {
      fontFamily: '"DM Sans", system-ui, sans-serif',
      fontSize: "0.875rem",
      fontWeight: 600,
      borderRadius: "8px",
      backgroundColor: "#e5853b",
      ":hover": { backgroundColor: "#d0752f" },
      ":focus": { backgroundColor: "#d0752f" },
      textTransform: "none",
    },
    formFieldInput: {
      borderRadius: "8px",
      border: "1px solid #ccc8c3",
      padding: "0.5rem 1rem",
      fontSize: "1rem",
    },
    socialButtonsBlockButton: {
      borderRadius: "8px",
      border: "1px solid #e3e0dd",
    },
    // Keep Clerk's footer link ("New here? Create an organization" /
    // "Already have an account? Log in") — it provides the cross-nav
    // between login and signup pages and is themed against our amber
    // accent via the colorPrimary variable above.
    footerActionLink: { color: "#e5853b" },
  },
};
