export const ssr = false;
import { get } from 'svelte/store';
import {user, token, isAuthenticated} from '$lib/stores/auth'


/** @type {import('./$types').PageLoad} */
export async function load({fetch, params}) {
  console.log(get(token))
  const res = await fetch('http://localhost:8080/api/peers', {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${get(token)}`,
    },
  });

  const data = await res.json();
  return {peers: data};
}
