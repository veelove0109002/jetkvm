/**
 * Temporary type shims to unblock TypeScript in environments without Node/npm installed.
 * Remove this file after installing dependencies (react, @types/react, vite) and type errors are resolved.
 */

// React module shim
declare module "react" {
  const React: any;
  export default React;

  export const useState: any;
  export const useEffect: any;
  export const useMemo: any;
  export const useCallback: any;
}

// React JSX runtime shim
declare module "react/jsx-runtime" {
  export const jsx: any;
  export const jsxs: any;
  export const Fragment: any;
}

// Allow any JSX intrinsic elements to avoid JSX namespace errors
declare namespace JSX {
  interface IntrinsicElements {
    [elemName: string]: any;
  }
}

// Vite client shim
declare module "vite/client" {}