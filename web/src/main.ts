import { mount } from 'svelte';
import './app.css';
import App from './App.svelte';

// The service worker only runs in production builds, so live reload stays simple.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js');
}

export default mount(App, { target: document.getElementById('app')! });
