import { mount } from "svelte";
import "./styles.css";
import App from "./App.svelte";
import { app } from "./lib/app.svelte";

app.init();

mount(App, { target: document.getElementById("app")! });
