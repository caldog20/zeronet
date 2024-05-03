import { writable } from 'svelte/store';

const isAuthenticated = writable(false);
const user = writable({});
const popupOpen = writable(false);
const error = writable();
const token = writable();
export {isAuthenticated, user, popupOpen, error, token};
