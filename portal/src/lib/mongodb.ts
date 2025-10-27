// src/lib/mongodb.ts
import { MongoClient } from 'mongodb';

const uri = `mongodb://admin:password123@localhost:27017`;
const client = new MongoClient(uri);

let isConnected = false;

export async function connectToDatabase() {
  if (!isConnected) {
    await client.connect();
    isConnected = true;
  }
  return client.db('hts-config');
}

export async function getHealthChecksCollection() {
  const db = await connectToDatabase();
  return db.collection('healthchecks');
}