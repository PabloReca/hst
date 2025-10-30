// src/lib/mongodb.ts
import { MongoClient } from 'mongodb';
import { MONGO_URI, MONGO_DATABASE } from 'astro:env/server';

const uri = MONGO_URI;
const database = MONGO_DATABASE;
const client = new MongoClient(uri);

let isConnected = false;

export async function connectToDatabase() {
  if (!isConnected) {
    await client.connect();
    isConnected = true;
  }
  return client.db(database);
}

export async function getHealthChecksCollection() {
  const db = await connectToDatabase();
  return db.collection('healthchecks');
}

export async function getLoadTestMetricsCollection() {
  const db = await connectToDatabase();
  return db.collection('loadtest_metrics');
}